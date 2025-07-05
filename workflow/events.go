package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType 定义事件类型
type EventType string

const (
	// Workflow 级别事件
	WorkflowStarted   EventType = "workflow.started"
	WorkflowCompleted EventType = "workflow.completed"

	// Node 级别事件
	NodeStarted   EventType = "node.started"
	NodeCompleted EventType = "node.completed"
)

// EventState 定义事件状态
type EventState string

const (
	StateSuccess  EventState = "success"
	StateFailed   EventState = "failed"
	StateCanceled EventState = "canceled"
)

// Event 统一事件结构
type Event struct {
	// 基础信息
	ID        string     // 事件唯一标识
	Type      EventType  // 事件类型
	State     EventState // 事件状态（仅适用于完成类事件）
	Timestamp time.Time  // 事件发生时间

	// 上下文引用
	WorkflowContext *WorkflowContext // 工作流上下文
	NodeContext     *NodeContext     // 节点上下文（node事件时非空）

	// 错误信息（失败状态时使用）
	Error error // 错误信息
}

func (e *Event) String() string {

	workflowID := e.WorkflowContext.Workflow.ID
	nodeID := ""
	if e.NodeContext != nil {
		nodeID = e.NodeContext.Node.ID
	}

	errMsg := ""
	if e.Error != nil {
		errMsg = e.Error.Error()
	}

	return fmt.Sprintf("%s %s:%s %s %s %s",
		e.Timestamp.Format("2006-01-02 15:04:05.000"),
		workflowID,
		nodeID,
		e.Type,
		e.State,
		errMsg)
}

// EventStore 事件存储器接口
type EventStore interface {
	// 基础操作
	Append(event Event) error
	Close() error

	// 迭代器接口
	Next(ctx context.Context) (Event, bool)
	Reset()

	// 批量操作
	AllEvents() []Event
	CountEvents() int

	// 状态查询
	IsClosed() bool
	CurrentIndex() int

	// 统一事件发射
	Emit(eventType EventType, state EventState, wc *WorkflowContext, nc *NodeContext, err error) error
}

// DefaultEventStore 默认事件存储器实现
type DefaultEventStore struct {
	events []Event
	index  int
	closed bool
	mu     sync.RWMutex
	cond   *sync.Cond
}

// NewDefaultEventStore 创建新的默认事件存储器
func NewDefaultEventStore() *DefaultEventStore {
	s := &DefaultEventStore{
		events: make([]Event, 0),
		index:  0,
		closed: false,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Append 添加事件
func (s *DefaultEventStore) Append(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("event store is closed")
	}

	s.events = append(s.events, event)
	s.cond.Broadcast() // 通知等待的消费者
	return nil
}

// Close 关闭存储
func (s *DefaultEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		s.cond.Broadcast() // 通知所有等待的消费者
	}
	return nil
}

// Next 获取下一个事件，支持上下文取消
func (s *DefaultEventStore) Next(ctx context.Context) (Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 循环等待直到有新事件或队列关闭
	for s.index >= len(s.events) && !s.closed {
		// 创建一个用于上下文取消的通道
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				s.cond.Broadcast() // 唤醒等待
				close(done)
			case <-done:
				// 正常结束
			}
		}()

		s.cond.Wait() // 释放锁并等待通知

		// 检查是否因上下文取消而唤醒
		select {
		case <-ctx.Done():
			close(done)
			return Event{}, false
		default:
			close(done)
			// 继续循环检查条件
		}
	}

	// 队列已关闭且没有更多事件
	if s.index >= len(s.events) {
		return Event{}, false
	}

	// 获取当前索引的事件并递增索引
	event := s.events[s.index]
	s.index++
	return event, true
}

// Reset 重置迭代器索引
func (s *DefaultEventStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index = 0
}

// AllEvents 获取所有事件
func (s *DefaultEventStore) AllEvents() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回事件的副本，避免外部修改
	result := make([]Event, len(s.events))
	copy(result, s.events)
	return result
}

// CountEvents 获取事件数量
func (s *DefaultEventStore) CountEvents() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.events)
}

// IsClosed 检查存储是否已关闭
func (s *DefaultEventStore) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.closed
}

// CurrentIndex 获取当前索引
func (s *DefaultEventStore) CurrentIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.index
}

// Emit 统一事件发射方法
func (s *DefaultEventStore) Emit(eventType EventType, state EventState, wc *WorkflowContext, nc *NodeContext, err error) error {
	event := Event{
		ID:              uuid.New().String(),
		Type:            eventType,
		State:           state,
		Timestamp:       time.Now(),
		WorkflowContext: wc,
		NodeContext:     nc,
		Error:           err,
	}

	return s.Append(event)
}
