package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	// TODO: trade log for logrus
	// log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	binName    = "tree-spotter"
	namespace  = "jan"
	helmFolder = "helm"
	dockerfile = "Dockerfile"
)

type docker struct {
	TLSverify string
	Host      string
	CertPath  string
}

func selectHelmBinary() (string, error) {
	switch os := runtime.GOOS; os {
	case "darwin":
		return "darwin.amd64", nil
	case "linux":
		return "", nil
	case "windows":
		return ".exe", nil
	default:
		return "", fmt.Errorf("operating system %s is not supported", os)
	}
}

func main() {
	dockerEnv, kubeconfig, minikubeIP, err := readEnvVars()
	check(err)

	helmBin, err := selectHelmBinary()
	check(err)

	cwd, err := os.Getwd()
	check(err)
	check(os.Chdir(fmt.Sprintf("%s/../", cwd)))

	buildDir, err := os.Getwd()

	version, err := loadVersion(fmt.Sprintf("%s/%s", buildDir, helmFolder))
	check(err)

	err = validateVersion(dockerEnv, binName, version)
	if err != nil {
		check(fmt.Errorf(
			"%s\nThe app version is defined in \"%s/%s/Chart.yaml\" in the Version field",
			err, buildDir, helmFolder))
	}

	check(build(buildDir))
	fmt.Println("")

	check(dockerBuild(dockerEnv, version))
	fmt.Println("")

	check(helmDeploy(kubeconfig, helmBin, buildDir))
	fmt.Println("")

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

func loadVersion(helmFolder string) (string, error) {
	f, err := ioutil.ReadFile(fmt.Sprintf("%s/Chart.yaml", helmFolder))
	if err != nil {
		return "",
			fmt.Errorf("[ERROR] read file \"%s\" error: \"%v\"", helmFolder, err)
	}
	version := struct{ Version string }{}
	err = yaml.Unmarshal(f, &version)
	if err != nil {
		return "",
			fmt.Errorf("[ERROR] unmarshal file \"%s\" error: \"%v\"", helmFolder, err)
	}
	return version.Version, nil
}

func collectVersionsLocalDocker(d docker, imageName string) ([]string, error) {

	out, err := runEnvRes(
		map[string]string{
			"DOCKER_TLS_VERIFY": d.TLSverify,
			"DOCKER_HOST":       d.Host,
			"DOCKER_CERT_PATH":  d.CertPath,
		},
		[]string{"docker", "images", imageName},
	)
	if err != nil {
		return []string{}, fmt.Errorf("load docker images error %s", err)
	}

	result := []string{}
	outLines := strings.Split(out, "\n")
	for _, line := range outLines {
		words := strings.Fields(line)
		if len(words) < 5 {
			continue
		}
		if words[0] == imageName {
			result = append(result, words[1])
		}
	}
	return result, nil
}

func validateVersion(d docker, imageName, version string) error {
	existingVersions, err := collectVersionsLocalDocker(d, imageName)
	if err != nil {
		return err
	}
	for _, exists := range existingVersions {
		if strings.Compare(version, exists) == 0 {
			return fmt.Errorf(
				"[ERROR] version \"%s\" already exists. The following versions exist \n %v",
				version,
				existingVersions,
			)
		}
	}
	return nil
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

	err = os.Rename(
		fmt.Sprintf("%s/%s", buildDir, binName),
		fmt.Sprintf("%s/app/%s", buildDir, binName),
	)

	if err != nil {
		return fmt.Errorf("[ERROR] failed to move executable")
	}
	return nil
}

func dockerBuild(d docker, version string) error {
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

func ensureNamespace(kubeconfig string) error {
	kcEnv := map[string]string{"KUBECONFIG": kubeconfig}
	err := runEnv(
		"HELM3", kcEnv, []string{"kubectl", "get", "namespace", namespace},
	)

	if err != nil {
		e := runEnv(
			"HELM3", kcEnv, []string{"kubectl", "create", "namespace", namespace},
		)
		if e != nil {
			return fmt.Errorf(
				"failed to create namespace \"%s\" with kubeconfig \"%s\" - error: \"%s\"",
				namespace, kubeconfig, e)
		}
	}
	return nil
}

func helmDeploy(kubeconfig, helmBin, buildDir string) error {
	// create namespace if it does not exist
	err := ensureNamespace(kubeconfig)
	if err != nil {
		return err
	}

	// Install service via helm binary (version v3.0.0-beta.2)
	// From version v3 onwards helm does not require tiller in cluster anymore
	return runEnv(
		"HELM3",
		map[string]string{
			"KUBECONFIG": kubeconfig,
		},
		[]string{fmt.Sprintf("%s/binaries/helm3%s", buildDir, helmBin),
			"upgrade", fmt.Sprintf("%s-%s", namespace, binName),
			fmt.Sprintf("%s/helm", buildDir),
			"--install",
			"--namespace", namespace,
			"--recreate-pods",
		},
	)
	// preferably this should be done using the alpine/helm docker image.
	// the relevant command has the form (replacing proper variables)
	// docker run -ti --rm \
	// -e KUBECONFIG="$HOME/.kube/config" \
	// -e DOCKER_TLS_VERIFY="1" \
	// -e DOCKER_HOST="tcp://192.168.0.11:2376" \
	// -e DOCKER_CERT_PATH="/mnt/c/wslConfigs/.minikube/certs" \
	// --bind $(pwd):/apps \
	// -v $HOME/.kube:/root/.kube \
	// -v $HOME/.helm:/root/.helm \
	// alpine/helm:3.0.0-beta.2 \
	// install /apps/helm test \
	// upgrade jan-tree-spotter /apps/helm \
	// --install \
	// --namespace jan \
	// --recreate-pods
}

func runTests(buildDir, minikubeIP string) error {
	time.Sleep(15 * time.Second)
	err := runEnv(
		"TEST",
		map[string]string{
			"MINIKUBE_IP": minikubeIP,
		},
		[]string{"go", "test",
			fmt.Sprintf("%s/interface_tests/...", buildDir),
			"-count", "1",
		},
	)
	if err != nil {
		return fmt.Errorf("tests failed. Check existing deployment and run the \"interface_tests\" manually!\n%s", err)
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

// runEnvRes executes a given command in a subprocess and returns the result.
// It breaks on errors (from stderr) and returns the error.
func runEnvRes(envVars map[string]string, cmd []string) (
	string, error,
) {
	if len(cmd) == 0 {
		return "", fmt.Errorf("command length is zero")
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = os.Environ()
	for name, value := range envVars {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", name, value))
	}

	outErrB, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmd \"%s\" failed - status code \"%s\", msg \"%s\"",
			cmd, err, string(outErrB))
	}
	return string(outErrB), nil
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logInfo(prefix, entry string) {
	log.Printf("%s | %s", prefix, entry)
}
