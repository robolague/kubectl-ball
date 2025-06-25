package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type mockCmdRunner struct {
	output []byte
	err    error
	stdin  *strings.Reader
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	run    func() error
}

func (m *mockCmdRunner) Output() ([]byte, error) { return m.output, m.err }
func (m *mockCmdRunner) Run() error {
	if m.run != nil {
		return m.run()
	}
	if m.stdout != nil {
		m.stdout.Write(m.output)
	}
	if m.err != nil && m.stderr != nil {
		m.stderr.WriteString(m.err.Error())
	}
	return m.err
}
func (m *mockCmdRunner) SetStdin(stdin *strings.Reader) { m.stdin = stdin }
func (m *mockCmdRunner) SetStdout(stdout *bytes.Buffer) { m.stdout = stdout }
func (m *mockCmdRunner) SetStderr(stderr *bytes.Buffer) { m.stderr = stderr }

type mockCommander struct {
	calls   []string
	outputs map[string]*mockCmdRunner
}

func (m *mockCommander) Command(name string, args ...string) CmdRunner {
	key := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, key)
	if runner, ok := m.outputs[key]; ok {
		return runner
	}
	return &mockCmdRunner{output: []byte{}, err: nil}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	configPathOrig := configPath
	defer func() { configPath = configPathOrig }()
	configPath = cfgPath

	cfg := Config{
		Clusters:  []string{"c1", "c2"},
		Namespace: "test-ns",
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}
	loaded, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if !reflect.DeepEqual(cfg, loaded) {
		t.Errorf("expected %v, got %v", cfg, loaded)
	}
}

func TestGetContexts_Success(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"kubectl config get-contexts -o=name": {output: []byte("ctx1\nctx2\n"), err: nil},
		},
	}
	contexts, err := getContexts(mc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"ctx1", "ctx2"}
	if !reflect.DeepEqual(contexts, want) {
		t.Errorf("expected %v, got %v", want, contexts)
	}
}

func TestGetContexts_Error(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"kubectl config get-contexts -o=name": {output: nil, err: errors.New("fail")},
		},
	}
	_, err := getContexts(mc)
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected error containing 'fail', got %v", err)
	}
}

func TestSelectClusters_Success(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"fzf --multi --prompt=Select Clusters > ": {output: []byte("c1\nc2\n"), err: nil},
		},
	}
	contexts := []string{"c1", "c2", "c3"}
	selected, err := selectClusters(mc, contexts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"c1", "c2"}
	if !reflect.DeepEqual(selected, want) {
		t.Errorf("expected %v, got %v", want, selected)
	}
}

func TestSelectClusters_Error(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"fzf --multi --prompt=Select Clusters > ": {output: nil, err: errors.New("fzf fail")},
		},
	}
	_, err := selectClusters(mc, []string{"a", "b"})
	if err == nil || !strings.Contains(err.Error(), "fzf fail") {
		t.Errorf("expected error containing 'fzf fail', got %v", err)
	}
}

func TestRunCommandInContext_Success(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"kubectl --context ctx get pods": {output: []byte("pod1\npod2\n"), err: nil},
		},
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var buf bytes.Buffer
	wg.Add(1)
	go runCommandInContext(mc, "ctx", []string{"get", "pods"}, "", "", "", &wg, &mu, &buf)
	wg.Wait()
	out := buf.String()
	if !strings.Contains(out, "pod1") || !strings.Contains(out, "ctx") {
		t.Errorf("expected output to contain pod1 and ctx, got: %s", out)
	}
}

func TestRunCommandInContext_Error(t *testing.T) {
	mc := &mockCommander{
		outputs: map[string]*mockCmdRunner{
			"kubectl --context ctx get pods": {output: []byte(""), err: errors.New("fail")},
		},
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var buf bytes.Buffer
	wg.Add(1)
	go runCommandInContext(mc, "ctx", []string{"get", "pods"}, "", "", "", &wg, &mu, &buf)
	wg.Wait()
	out := buf.String()
	if !strings.Contains(out, "Error") || !strings.Contains(out, "fail") {
		t.Errorf("expected error output, got: %s", out)
	}
}

// Note: getContexts and other exec.Command-based functions are not tested here due to lack of dependency injection.
