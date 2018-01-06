package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestExplorer struct {
	Type         string
	StartCounter int
}

var _ Explorer = (*TestExplorer)(nil)

func (e *TestExplorer) GetType() string {
	return e.Type
}

func (e *TestExplorer) Start() {
	e.StartCounter++
}

func (e *TestExplorer) Stop() {}

func (e *TestExplorer) WaitForShutdown() {}

func TestStartAll(t *testing.T) {
	// Arrange
	e1 := &TestExplorer{Type: "test1"}
	e2 := &TestExplorer{Type: "test2"}
	e := explorer{
		explorers: []Explorer{e1, e2},
	}

	// Act
	e.Start()

	// Assert
	assert.Equal(t, 1, e1.StartCounter)
	assert.Equal(t, 1, e2.StartCounter)
}

func TestStartAll2(t *testing.T) {
	// Arrange
	e1 := &TestExplorer{Type: "test1"}
	e2 := &TestExplorer{Type: "test2"}
	e := explorer{
		explorers:        []Explorer{e1, e2},
		explorersToStart: []string{"test1", "test2"},
	}

	// Act
	e.Start()

	// Assert
	assert.Equal(t, 1, e1.StartCounter)
	assert.Equal(t, 1, e2.StartCounter)
}

func TestStartFirst(t *testing.T) {
	// Arrange
	e1 := &TestExplorer{Type: "test1"}
	e2 := &TestExplorer{Type: "test2"}
	e := explorer{
		explorers:        []Explorer{e1, e2},
		explorersToStart: []string{"test1"},
	}

	// Act
	e.Start()

	// Assert
	assert.Equal(t, 1, e1.StartCounter)
	assert.Equal(t, 0, e2.StartCounter)
}

func TestStartNil(t *testing.T) {
	// Arrange
	e1 := &TestExplorer{Type: "test1"}
	e2 := &TestExplorer{Type: "test2"}
	e := explorer{
		explorers:        []Explorer{e1, e2},
		explorersToStart: nil,
	}

	// Act
	e.Start()

	// Assert
	assert.Equal(t, 1, e1.StartCounter)
	assert.Equal(t, 1, e2.StartCounter)
}
