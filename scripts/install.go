package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	// log "github.com/sirupsen/logrus"
)

const (
	binName = "tree-spotter"
	// TODO: pull version from chart and add validate version logic
	version   = "0.1.0"
	namespace = "jan"

	dockerfile = "Dockerfile"
)

type docker struct {
	TLSverify string
	Host      string
	CertPath  string
}

func main() {
	dockerEnv, kubeconfig, minikubeIP, err := readEnvVars()
	check(err)

	// TODO read flags

	cwd, err := os.Getwd()
	check(err)
	check(os.Chdir(fmt.Sprintf("%s/../", cwd)))

	buildDir, err := os.Getwd()
	check(err)

	check(build(buildDir))
	fmt.Println("")

	check(dockerBuild(dockerEnv))
	fmt.Println("")

	check(helmDeploy(kubeconfig, buildDir))
	fmt.Println("")

	time.Sleep(10 * time.Second)
	check(runTests(buildDir, minikubeIP))
	fmt.Println("")

	os.Chdir(cwd)
}

func readEnvVars() (docker, string, string, error) {
	kc := os.Getenv("KUBECONFIG")
	d := docker{}
	d.TLSverify = os.Getenv("DOCKER_TLS_VERIFY")
	d.Host = os.Getenv("DOCKER_HOST")
	d.CertPath = os.Getenv("DOCKER_CERT_PATH")
	ip := os.Getenv("MINIKUBE_IP")

	if d.TLSverify == "" ||
		d.Host == "" ||
		d.CertPath == "" {
		return docker{}, "", "", fmt.Errorf(`
[ERROR] docker environment must be set to minikube environment.
The env variables \"DOCKER_TLS_VERIFY\", \"DOCKER_HOST\" and \"DOCKER_CERT_PATH\" must be set`,
		)
	}
	if kc == "" {
		return docker{}, "", "",
			fmt.Errorf("[ERROR] env variable \"KUBECONFIG\" must be set and it must point to the minikube cluster")
	}
	if ip == "" {
		return docker{}, "", "", fmt.Errorf("[ERROR] env variable MINIKUBE_IP must be set")
	}
	return d, kc, ip, nil
}

func build(buildDir string) error {
	logInfo("BUILD", fmt.Sprintf("building go app in %s", buildDir))

	// Build a statically linked binary that can live in a scratch container
	err := runEnv(
		"BUILD",
		map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
		[]string{"go", "build", "-a", "-installsuffix", "cgo", "-o", binName, "."},
	)
	if err != nil {
		return err
	}
	logInfo("BUILD", "success")

	err = runEnv(
		"MOVE",
		map[string]string{},
		[]string{"mv", binName, "./app"},
	)
	if err != nil {
		return err
	}
	return nil
}

func dockerBuild(d docker) error {
	return runEnv(
		"DOCKER",
		map[string]string{
			"DOCKER_TLS_VERIFY": d.TLSverify,
			"DOCKER_HOST":       d.Host,
			"DOCKER_CERT_PATH":  d.CertPath,
		},
		[]string{
			"docker", "build",
			"-t", fmt.Sprintf("%s:%s", binName, version),
			"-f", dockerfile, "."},
	)
}

func helmDeploy(kubeconfig, buildDir string) error {
	// create namespace if it does not exist
	err := runEnv(
		"HELM3",
		map[string]string{"KUBECONFIG": kubeconfig},
		[]string{"kubectl", "get", "namespace", namespace},
	)
	if err != nil {
		e := runEnv(
			"HELM3",
			map[string]string{"KUBECONFIG": kubeconfig},
			[]string{"kubectl", "create", "namespace", namespace},
		)
		if e != nil {
			return fmt.Errorf(
				"failed to create namespace \"%s\" with kubeconfig \"%s\" - error: \"%s\"",
				namespace, kubeconfig, e)
		}
	}

	// Install service via helm binary (version v3.0.0-beta.3)
	// From version v3 onwards helm does not require tiller in cluster anymore
	return runEnv(
		"HELM3",
		map[string]string{
			"KUBECONFIG": kubeconfig,
		},
		[]string{fmt.Sprintf("%s/binaries/helm", buildDir),
			"upgrade", fmt.Sprintf("%s-%s", namespace, binName),
			"./helm",
			"--install",
			"--recreate-pods",
			"--namespace", namespace,
		},
	)
}

func runTests(buildDir, minikubeIP string) error {
	err := runEnv(
		"TEST",
		map[string]string{
			"MINIKUBE_IP": minikubeIP,
		},
		[]string{"go", "test", fmt.Sprintf("%s/interface_tests/...", buildDir)},
	)
	if err != nil {
		return fmt.Errorf("tests failed. Check existing deployment!\n%s", err)
	}
	return nil
}

// runEnv executes a given command in a subprocess and pipes all occurring outputs
// to stdout. It breaks on errors (from stderr)
func runEnv(prefix string, envVars map[string]string, cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command length is zero")
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = os.Environ()
	for name, value := range envVars {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", name, value))
	}

	cReader, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf(
			"error \"%s\" creating StdoutPipe for cmd: \"%s\"",
			err, strings.Join(cmd, " "))
	}

	scanner := bufio.NewScanner(cReader)
	go func() {
		for scanner.Scan() {
			logInfo(prefix, scanner.Text())
		}
	}()
	err = c.Start()
	if err != nil {
		return fmt.Errorf("error \"%s\" starting cmd \"%s\"", err, strings.Join(cmd, " "))
	}
	err = c.Wait()
	if err != nil {
		return fmt.Errorf("error \"%s\" waiting for cmd \"%s\"", err, strings.Join(cmd, " "))
	}
	return nil
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logInfo(prefix, entry string) {
	log.Printf("%s | %s", prefix, entry)
}
