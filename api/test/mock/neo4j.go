// test/mock/neo4j.go
package mock

import (
	"net/url"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/mock"
)

// MockDriver is a mock implementation of neo4j.Driver
type MockDriver struct {
	mock.Mock
}

func (m *MockDriver) NewSession(config neo4j.SessionConfig) neo4j.Session {
	args := m.Called(config)
	return args.Get(0).(neo4j.Session)
}

func (m *MockDriver) VerifyConnectivity() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDriver) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDriver) Target() url.URL {
	args := m.Called()
	return args.Get(0).(url.URL)
}

func (m *MockDriver) IsEncrypted() bool {
	args := m.Called()
	return args.Bool(0)
}

// MockSession is a mock implementation of neo4j.Session
type MockSession struct {
	mock.Mock
}

func (m *MockSession) Run(cypher string, params map[string]any, configurers ...func(*neo4j.TransactionConfig)) (neo4j.Result, error) {
	args := m.Called(cypher, params, configurers)
	return args.Get(0).(neo4j.Result), args.Error(1)
}

func (m *MockSession) ReadTransaction(work neo4j.TransactionWork, configurers ...func(*neo4j.TransactionConfig)) (interface{}, error) {
	args := m.Called(work, configurers)
	return args.Get(0), args.Error(1)
}

func (m *MockSession) WriteTransaction(work neo4j.TransactionWork, configurers ...func(*neo4j.TransactionConfig)) (interface{}, error) {
	args := m.Called(work, configurers)
	return args.Get(0), args.Error(1)
}

func (m *MockSession) BeginTransaction(configurers ...func(*neo4j.TransactionConfig)) (neo4j.Transaction, error) {
	args := m.Called(configurers)
	return args.Get(0).(neo4j.Transaction), args.Error(1)
}

func (m *MockSession) LastBookmark() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSession) LastBookmarks() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockSession) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockTransaction is a mock implementation of neo4j.Transaction
type MockTransaction struct {
	mock.Mock
}

func (m *MockTransaction) Run(cypher string, params map[string]any) (neo4j.Result, error) {
	args := m.Called(cypher, params)
	result, _ := args.Get(0).(neo4j.Result)
	return result, args.Error(1)
}

func (m *MockTransaction) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTransaction) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTransaction) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockResult is a mock implementation of neo4j.Result
type MockResult struct {
	mock.Mock
}

func (m *MockResult) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockResult) Record() *neo4j.Record {
	args := m.Called()
	return args.Get(0).(*neo4j.Record)
}

func (m *MockResult) Consume() (neo4j.ResultSummary, error) {
	args := m.Called()
	return args.Get(0).(neo4j.ResultSummary), args.Error(1)
}

func (m *MockResult) Summary() (neo4j.ResultSummary, error) {
	args := m.Called()
	return args.Get(0).(neo4j.ResultSummary), args.Error(1)
}

func (m *MockResult) Collect() ([]*neo4j.Record, error) {
	args := m.Called()
	return args.Get(0).([]*neo4j.Record), args.Error(1)
}

func (m *MockResult) Keys() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockResult) Peek() (*neo4j.Record, error) {
	args := m.Called()
	return args.Get(0).(*neo4j.Record), args.Error(1)
}

func (m *MockResult) Err() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResult) NextRecord() (**neo4j.Record, error) {
	args := m.Called()
	return args.Get(0).(**neo4j.Record), args.Error(1)
}

func (m *MockResult) Single() (interface{}, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}

func (m *MockResult) KeysSummary() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockResult) NextNeo4jRecord() (neo4j.Record, error) {
	args := m.Called()
	return args.Get(0).(neo4j.Record), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordSlice() ([]neo4j.Record, error) {
	args := m.Called()
	return args.Get(0).([]neo4j.Record), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordSliceMap() ([]map[string]interface{}, error) {
	args := m.Called()
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordMap() (map[string]interface{}, error) {
	args := m.Called()
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordValue() (interface{}, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordValues() ([]interface{}, error) {
	args := m.Called()
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockResult) NextNeo4jRecordValuesMap() (map[string]interface{}, error) {
	args := m.Called()
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// ResultSummary is a mock implementation of neo4j.ResultSummary
type ResultSummary struct {
	mock.Mock
}

func (m *ResultSummary) Counters() neo4j.Counters {
	args := m.Called()
	return args.Get(0).(neo4j.Counters)
}

func (m *ResultSummary) Server() neo4j.ServerInfo {
	args := m.Called()
	return args.Get(0).(neo4j.ServerInfo)
}

func (m *ResultSummary) Notifications() []neo4j.Notification {
	args := m.Called()
	return args.Get(0).([]neo4j.Notification)
}

func (m *ResultSummary) ResultAvailableAfter() neo4j.Duration {
	args := m.Called()
	return args.Get(0).(neo4j.Duration)
}

func (m *ResultSummary) ResultConsumedAfter() neo4j.Duration {
	args := m.Called()
	return args.Get(0).(neo4j.Duration)
}
