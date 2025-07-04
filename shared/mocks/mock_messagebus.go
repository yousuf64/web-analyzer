// Code generated by MockGen. DO NOT EDIT.
// Source: shared/messagebus (interfaces: MessageBusInterface)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_messagebus.go -package=mocks . MessageBusInterface
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	messagebus "shared/messagebus"

	nats "github.com/nats-io/nats.go"
	gomock "go.uber.org/mock/gomock"
)

// MockMessageBusInterface is a mock of MessageBusInterface interface.
type MockMessageBusInterface struct {
	ctrl     *gomock.Controller
	recorder *MockMessageBusInterfaceMockRecorder
	isgomock struct{}
}

// MockMessageBusInterfaceMockRecorder is the mock recorder for MockMessageBusInterface.
type MockMessageBusInterfaceMockRecorder struct {
	mock *MockMessageBusInterface
}

// NewMockMessageBusInterface creates a new mock instance.
func NewMockMessageBusInterface(ctrl *gomock.Controller) *MockMessageBusInterface {
	mock := &MockMessageBusInterface{ctrl: ctrl}
	mock.recorder = &MockMessageBusInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMessageBusInterface) EXPECT() *MockMessageBusInterfaceMockRecorder {
	return m.recorder
}

// PublishAnalyzeMessage mocks base method.
func (m_2 *MockMessageBusInterface) PublishAnalyzeMessage(ctx context.Context, m messagebus.AnalyzeMessage) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "PublishAnalyzeMessage", ctx, m)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishAnalyzeMessage indicates an expected call of PublishAnalyzeMessage.
func (mr *MockMessageBusInterfaceMockRecorder) PublishAnalyzeMessage(ctx, m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishAnalyzeMessage", reflect.TypeOf((*MockMessageBusInterface)(nil).PublishAnalyzeMessage), ctx, m)
}

// PublishJobUpdate mocks base method.
func (m_2 *MockMessageBusInterface) PublishJobUpdate(ctx context.Context, m messagebus.JobUpdateMessage) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "PublishJobUpdate", ctx, m)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishJobUpdate indicates an expected call of PublishJobUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) PublishJobUpdate(ctx, m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishJobUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).PublishJobUpdate), ctx, m)
}

// PublishSubTaskUpdate mocks base method.
func (m_2 *MockMessageBusInterface) PublishSubTaskUpdate(ctx context.Context, m messagebus.SubTaskUpdateMessage) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "PublishSubTaskUpdate", ctx, m)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishSubTaskUpdate indicates an expected call of PublishSubTaskUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) PublishSubTaskUpdate(ctx, m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishSubTaskUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).PublishSubTaskUpdate), ctx, m)
}

// PublishTaskStatusUpdate mocks base method.
func (m_2 *MockMessageBusInterface) PublishTaskStatusUpdate(ctx context.Context, m messagebus.TaskStatusUpdateMessage) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "PublishTaskStatusUpdate", ctx, m)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishTaskStatusUpdate indicates an expected call of PublishTaskStatusUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) PublishTaskStatusUpdate(ctx, m any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishTaskStatusUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).PublishTaskStatusUpdate), ctx, m)
}

// SubscribeToAnalyzeMessage mocks base method.
func (m *MockMessageBusInterface) SubscribeToAnalyzeMessage(handler func(context.Context, *nats.Msg)) (*nats.Subscription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeToAnalyzeMessage", handler)
	ret0, _ := ret[0].(*nats.Subscription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeToAnalyzeMessage indicates an expected call of SubscribeToAnalyzeMessage.
func (mr *MockMessageBusInterfaceMockRecorder) SubscribeToAnalyzeMessage(handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeToAnalyzeMessage", reflect.TypeOf((*MockMessageBusInterface)(nil).SubscribeToAnalyzeMessage), handler)
}

// SubscribeToJobUpdate mocks base method.
func (m *MockMessageBusInterface) SubscribeToJobUpdate(handler func(context.Context, *nats.Msg)) (*nats.Subscription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeToJobUpdate", handler)
	ret0, _ := ret[0].(*nats.Subscription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeToJobUpdate indicates an expected call of SubscribeToJobUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) SubscribeToJobUpdate(handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeToJobUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).SubscribeToJobUpdate), handler)
}

// SubscribeToSubTaskUpdate mocks base method.
func (m *MockMessageBusInterface) SubscribeToSubTaskUpdate(handler func(context.Context, *nats.Msg)) (*nats.Subscription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeToSubTaskUpdate", handler)
	ret0, _ := ret[0].(*nats.Subscription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeToSubTaskUpdate indicates an expected call of SubscribeToSubTaskUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) SubscribeToSubTaskUpdate(handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeToSubTaskUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).SubscribeToSubTaskUpdate), handler)
}

// SubscribeToTaskStatusUpdate mocks base method.
func (m *MockMessageBusInterface) SubscribeToTaskStatusUpdate(handler func(context.Context, *nats.Msg)) (*nats.Subscription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscribeToTaskStatusUpdate", handler)
	ret0, _ := ret[0].(*nats.Subscription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubscribeToTaskStatusUpdate indicates an expected call of SubscribeToTaskStatusUpdate.
func (mr *MockMessageBusInterfaceMockRecorder) SubscribeToTaskStatusUpdate(handler any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscribeToTaskStatusUpdate", reflect.TypeOf((*MockMessageBusInterface)(nil).SubscribeToTaskStatusUpdate), handler)
}
