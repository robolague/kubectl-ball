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

	"gopkg.in/yaml.v2"
)

type Config struct {
	Clusters  []string `yaml:"clusters"`
	Namespace string   `yaml:"namespace"`
}

var configPath = filepath.Join(os.Getenv("HOME"), ".kubectl-ball", "config.yaml")

func checkFzf() error {
	_, err := exec.LookPath("fzf")
	if err != nil {
		return fmt.Errorf(`"fzf" not found. Install it:
  macOS:   brew install fzf
  Ubuntu:  sudo apt install fzf
  Docs:    https://github.com/junegunn/fzf`)
	}
	return nil
}

func getContexts(merge bool) ([]string, error) {
	if merge {
		cmd := exec.Command("kubectl", "config", "view", "--flatten", "--minify", "-o=jsonpath={.contexts[*].name}")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		return strings.Fields(string(output)), nil
	}
	cmd := exec.Command("kubectl", "config", "get-contexts", "-o=name")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

func selectClusters(contexts []string) ([]string, error) {
	cmd := exec.Command("fzf", "--multi", "--prompt=Select Clusters > ")
	cmd.Stdin = strings.NewReader(strings.Join(contexts, "\n"))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fzf selection error: %w", err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
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

func runCommandInContext(context string, args []string, namespace, grep, outputFormat string, wg *sync.WaitGroup, mu *sync.Mutex, results *bytes.Buffer) {
	defer wg.Done()

	cmdArgs := append([]string{"--context", context}, args...)
	if namespace != "" {
		cmdArgs = append(cmdArgs, "-n", namespace)
	}
	if outputFormat != "" {
		cmdArgs = append(cmdArgs, "-o", outputFormat)
	}

	cmd := exec.Command("kubectl", cmdArgs...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

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
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Usage: kubectl ball [--select] [--merge-kubeconfigs] [--grep pattern] [--format json|yaml|wide|table] [-n ns] <kubectl args>")
		os.Exit(1)
	}

	var (
		selectFlag, mergeKubeconfigs bool
		namespace, grepPattern, outputFormat string
		kubectlArgs []string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--select":
			selectFlag = true
		case "--merge-kubeconfigs":
			mergeKubeconfigs = true
		case "--grep":
			if i+1 < len(args) { grepPattern = args[i+1]; i++ }
		case "--format":
			if i+1 < len(args) { outputFormat = args[i+1]; i++ }
		case "-n", "--namespace":
			if i+1 < len(args) { namespace = args[i+1]; i++ }
		default:
			kubectlArgs = append(kubectlArgs, args[i])
		}
	}

	var config Config
	var err error

	if selectFlag || _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := checkFzf(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		contexts, err := getContexts(mergeKubeconfigs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get contexts: %v\n", err)
			os.Exit(1)
		}
		selected, err := selectClusters(contexts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cluster selection failed: %v\n", err)
			os.Exit(1)
		}
		config = Config{Clusters: selected, Namespace: namespace}
		saveConfig(config)
	} else {
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

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results bytes.Buffer

	for _, context := range config.Clusters {
		wg.Add(1)
		go runCommandInContext(context, kubectlArgs, config.Namespace, grepPattern, outputFormat, &wg, &mu, &results)
	}

	wg.Wait()
	fmt.Print(results.String())
}
