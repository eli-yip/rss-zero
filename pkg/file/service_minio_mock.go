package file

import "io"

type MockMinio struct{}

func (m *MockMinio) SaveStream(string, io.ReadCloser, int64) error { return nil }
func (m *MockMinio) Get(string) (io.ReadCloser, error)             { return nil, nil }
func (m *MockMinio) AssetsDomain() string                          { return "" }
