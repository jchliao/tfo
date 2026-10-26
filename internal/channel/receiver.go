package channel

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
)

// NoteCreator 笔记创建接口（解耦对 note.Service 的依赖）
type NoteCreator interface {
	Create(ctx context.Context, content string) (interface{}, error)
}

// ImageSaver 把通道收到的图片落盘，并返回可直接插入笔记正文的 Markdown 片段。
// 由调用方注入（通常是 attachment.Store），通道层不关心具体存储位置。
type ImageSaver func(data []byte) (markdown string, err error)

// ReceiptText 回执确认文本，可自定义
var ReceiptText = "✅ 已记录"

// Receiver 统一消息接收管理器
type Receiver struct {
	mu          sync.RWMutex
	adapters    []MessageAdapter
	adapterMap  map[string]MessageAdapter // name → adapter，用于回复路由
	noteCreator NoteCreator
	imageSaver  ImageSaver
	enableReply bool // 是否在创建笔记后自动回复
}

// NewReceiver 创建消息接收器
func NewReceiver(noteCreator NoteCreator) *Receiver {
	return &Receiver{
		noteCreator: noteCreator,
		adapterMap:  make(map[string]MessageAdapter),
		enableReply: true,
	}
}

// SetImageSaver 注入图片存储实现；未注入时通道消息里的图片会被丢弃。
func (r *Receiver) SetImageSaver(saver ImageSaver) {
	r.mu.Lock()
	r.imageSaver = saver
	r.mu.Unlock()
}

// SetEnableReply 设置是否自动回复
func (r *Receiver) SetEnableReply(enable bool) {
	r.mu.Lock()
	r.enableReply = enable
	r.mu.Unlock()
}

// RegisterAdapter 注册一个消息适配器，自动注入 handleMessage 回调
func (r *Receiver) RegisterAdapter(adapter MessageAdapter) {
	adapter.SetMessageHandler(r.handleMessage)
	r.mu.Lock()
	r.adapters = append(r.adapters, adapter)
	r.adapterMap[adapter.Name()] = adapter
	r.mu.Unlock()
}

// StartAll 启动所有已注册的适配器
func (r *Receiver) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []string
	for _, a := range r.adapters {
		if err := a.Start(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", a.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("start adapters: %s", strings.Join(errs, "; "))
	}
	return nil
}

// StopAll 优雅关闭所有适配器
func (r *Receiver) StopAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []string
	for _, a := range r.adapters {
		if err := a.Stop(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", a.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop adapters: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetAdapterStates 返回所有适配器的连接状态
func (r *Receiver) GetAdapterStates() map[string]ConnState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make(map[string]ConnState, len(r.adapters))
	for _, a := range r.adapters {
		states[a.Name()] = a.State()
	}
	return states
}

// handleMessage 统一消息处理：保存图片 → 格式化内容 → 创建笔记
func (r *Receiver) handleMessage(ctx context.Context, msg *IncomingMessage) error {
	if msg == nil {
		return nil
	}

	content, imageCount := r.buildContent(msg)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	_, err := r.noteCreator.Create(ctx, content)
	if err != nil {
		log.Printf("[receiver] create note from %s: %v", msg.ChannelType, err)
		return err
	}

	log.Printf("[receiver] note created from %s, sender=%s, images=%d", msg.ChannelType, msg.SenderID, imageCount)

	// 自动回复确认
	r.mu.RLock()
	reply := r.enableReply
	adapter := r.adapterMap[msg.ChannelType]
	r.mu.RUnlock()

	log.Printf("[receiver] reply check: enableReply=%v, adapter=%v, replyToken=%q", reply, adapter != nil, msg.ReplyToken)
	if reply && adapter != nil && msg.ReplyToken != "" {
		text := ReceiptText
		if imageCount > 0 {
			text = fmt.Sprintf("%s（含 %d 张图片）", ReceiptText, imageCount)
		}
		if err := adapter.Reply(ctx, msg.ReplyToken, text); err != nil {
			log.Printf("[receiver] reply to %s failed: %v", msg.ChannelType, err)
			// 回复失败不影响笔记创建结果
		} else {
			log.Printf("[receiver] reply sent to %s successfully", msg.ChannelType)
		}
	}

	return nil
}

// buildContent 把文本与图片合成为笔记正文，返回正文和成功保存的图片数。
func (r *Receiver) buildContent(msg *IncomingMessage) (string, int) {
	r.mu.RLock()
	saver := r.imageSaver
	r.mu.RUnlock()

	var sb strings.Builder
	if text := strings.TrimSpace(msg.Content); text != "" {
		sb.WriteString(text)
	}

	saved := 0
	for _, img := range msg.Images {
		if len(img.Data) == 0 {
			continue
		}
		if saver == nil {
			log.Printf("[receiver] drop image from %s: image saver is not configured", msg.ChannelType)
			continue
		}
		markdown, err := saver(img.Data)
		if err != nil {
			log.Printf("[receiver] save image from %s: %v", msg.ChannelType, err)
			continue
		}
		if strings.TrimSpace(markdown) == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(markdown)
		saved++
	}

	if sb.Len() == 0 {
		return "", 0
	}
	return formatMessageContent(msg.ChannelType, sb.String()), saved
}

// formatMessageContent 格式化外部消息为笔记内容
func formatMessageContent(channelType, content string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(content))

	// 附加来源标签
	tag := "#" + channelType
	sb.WriteString("\n\n")
	sb.WriteString(tag)

	return sb.String()
}
