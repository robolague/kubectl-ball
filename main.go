// kubectl-ball: A kubectl plugin to operate across clusters with fzf, formatting, grepping, and kubeconfig merging
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Clusters  []string `yaml:"clusters"`
	Namespace string   `yaml:"namespace"`
}

var configPath = filepath.Join(os.Getenv("HOME"), ".kubectl-ball", "config.yaml")

type CmdRunner interface {
	Output() ([]byte, error)
	Run() error
	SetStdin(stdin *strings.Reader)
	SetStdout(stdout *bytes.Buffer)
	SetStderr(stderr *bytes.Buffer)
}

type Commander interface {
	Command(name string, args ...string) CmdRunner
}

type realCommander struct{}

func (realCommander) Command(name string, args ...string) CmdRunner {
	return &realCmdWrapper{cmd: exec.Command(name, args...)}
}

type realCmdWrapper struct {
	cmd *exec.Cmd
}

func (r *realCmdWrapper) Output() ([]byte, error)        { return r.cmd.Output() }
func (r *realCmdWrapper) Run() error                     { return r.cmd.Run() }
func (r *realCmdWrapper) SetStdin(stdin *strings.Reader) { r.cmd.Stdin = stdin }
func (r *realCmdWrapper) SetStdout(stdout *bytes.Buffer) { r.cmd.Stdout = stdout }
func (r *realCmdWrapper) SetStderr(stderr *bytes.Buffer) { r.cmd.Stderr = stderr }

func checkFzf(commander Commander) error {
	_, err := exec.LookPath("fzf")
	if err != nil {
		return fmt.Errorf(`"fzf" not found. Install it:
  macOS:   brew install fzf
  Ubuntu:  sudo apt install fzf
  Docs:    https://github.com/junegunn/fzf`)
	}
	return nil
}

func getContexts(commander Commander) ([]string, error) {
	cmd := commander.Command("kubectl", "config", "get-contexts", "-o=name")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

func selectClusters(commander Commander, contexts []string) ([]string, error) {
	// If there's only one context, automatically select it
	if len(contexts) == 1 {
		return contexts, nil
	}

	cmd := commander.Command("fzf", "--multi", "--prompt=Select Clusters > ")
	cmd.SetStdin(strings.NewReader(strings.Join(contexts, "\n")))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fzf selection error: %w", err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

func getNamespaces(commander Commander, context string) ([]string, error) {
	cmd := commander.Command("kubectl", "--context", context, "get", "namespaces", "-o=name")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Remove "namespace/" prefix from each namespace
	namespaces := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, ns := range namespaces {
		namespaces[i] = strings.TrimPrefix(ns, "namespace/")
	}
	return namespaces, nil
}

func selectNamespace(commander Commander, namespaces []string) (string, error) {
	// If there's only one namespace, automatically select it
	if len(namespaces) == 1 {
		return namespaces[0], nil
	}

	cmd := commander.Command("fzf", "--prompt=Select Namespace > ")
	cmd.SetStdin(strings.NewReader(strings.Join(namespaces, "\n")))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("fzf selection error: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func saveConfig(config Config) error {
	os.MkdirAll(filepath.Dir(configPath), 0700)
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func loadConfig() (Config, error) {
	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}

func runCommandInContext(commander Commander, context string, args []string, namespace, grep, outputFormat string, wg *sync.WaitGroup, mu *sync.Mutex, results *bytes.Buffer) {
	defer wg.Done()

	cmdArgs := append([]string{"--context", context}, args...)
	if namespace != "" {
		cmdArgs = append(cmdArgs, "-n", namespace)
	}
	if outputFormat != "" {
		cmdArgs = append(cmdArgs, "-o", outputFormat)
	}

	cmd := commander.Command("kubectl", cmdArgs...)
	var out, stderr bytes.Buffer
	cmd.SetStdout(&out)
	cmd.SetStderr(&stderr)

	err := cmd.Run()
	outputStr := out.String()

	if grep != "" {
		lines := []string{}
		for _, line := range strings.Split(outputStr, "\n") {
			if strings.Contains(line, grep) {
				lines = append(lines, line)
			}
		}
		if len(lines) == 0 {
			return // Skip this cluster if no match
		}
		outputStr = strings.Join(lines, "\n")
	}

	mu.Lock()
	defer mu.Unlock()
	results.WriteString(fmt.Sprintf("\n===== [%s] =====\n", context))
	if err != nil {
		results.WriteString(fmt.Sprintf("Error: %v\n%s\n", err, stderr.String()))
	} else {
		results.WriteString(outputStr + "\n")
	}
}

func main() {
	commander := realCommander{}
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: kubectl ball [--select] [--select-namespace] [--grep pattern] [--format json|yaml|wide|table] [-n ns] <kubectl args>")
		os.Exit(1)
	}

	var (
		selectFlag, selectNamespaceFlag      bool
		namespace, grepPattern, outputFormat string
		kubectlArgs                          []string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--select":
			selectFlag = true
		case "--select-namespace":
			selectNamespaceFlag = true
		case "--grep":
			if i+1 < len(args) {
				grepPattern = args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(args) {
				outputFormat = args[i+1]
				i++
			}
		case "-n", "--namespace":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		default:
			kubectlArgs = append(kubectlArgs, args[i])
		}
	}

	var config Config
	var err error

	if selectFlag {
		// Need to select clusters
	} else if _, err = os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, need to select clusters
	} else {
		// Config file exists, load it
		config, err = loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		if namespace != "" {
			config.Namespace = namespace
			saveConfig(config)
		}
	}

	// If we need to select clusters (either --select flag or no config file)
	if selectFlag || config.Clusters == nil {
		if err := checkFzf(commander); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		contexts, err := getContexts(commander)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get contexts: %v\n", err)
			os.Exit(1)
		}
		selected, err := selectClusters(commander, contexts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cluster selection failed: %v\n", err)
			os.Exit(1)
		}
		config = Config{Clusters: selected, Namespace: namespace}
		saveConfig(config)
	}

	// Handle namespace selection
	if selectNamespaceFlag {
		if err := checkFzf(commander); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Use the first cluster to get namespaces (assuming all clusters have similar namespaces)
		if len(config.Clusters) == 0 {
			fmt.Fprintln(os.Stderr, "No clusters selected. Please select clusters first.")
			os.Exit(1)
		}

		namespaces, err := getNamespaces(commander, config.Clusters[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get namespaces: %v\n", err)
			os.Exit(1)
		}

		selectedNamespace, err := selectNamespace(commander, namespaces)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Namespace selection failed: %v\n", err)
			os.Exit(1)
		}

		config.Namespace = selectedNamespace
		saveConfig(config)
	}

	// If no kubectl arguments provided, just save the selection and exit
	if len(kubectlArgs) == 0 {
		if selectFlag {
			fmt.Println("Selection saved successfully!")
			fmt.Printf("Selected clusters: %s\n", strings.Join(config.Clusters, ", "))
			return
		}
		if selectNamespaceFlag {
			fmt.Println("Selection saved successfully!")
			fmt.Printf("Selected namespace: %s\n", config.Namespace)
			return
		}
		// If no flags and no args, show usage
		fmt.Println("Usage: kubectl ball [--select] [--select-namespace] [--grep pattern] [--format json|yaml|wide|table] [-n ns] <kubectl args>")
		fmt.Println("Examples:")
		fmt.Println("  kubectl ball --select                    # Select clusters and save")
		fmt.Println("  kubectl ball --select-namespace          # Select namespace and save")
		fmt.Println("  kubectl ball get pods                    # Run 'get pods' across selected clusters")
		fmt.Println("  kubectl ball get pods --grep running     # Run 'get pods' and grep for 'running'")
		os.Exit(1)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results bytes.Buffer

	for _, context := range config.Clusters {
		wg.Add(1)
		go runCommandInContext(commander, context, kubectlArgs, config.Namespace, grepPattern, outputFormat, &wg, &mu, &results)
	}

	wg.Wait()
	fmt.Print(results.String())
}
