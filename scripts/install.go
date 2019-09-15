package main

import (
	"dev/ecosia_intro/scripts/helm"
	"dev/ecosia_intro/scripts/process"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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

func main() {
	dockerEnv, kubeconfig, minikubeIP, err := readEnvVars()
	check(err)

	cwd, err := os.Getwd()
	check(err)
	check(os.Chdir(fmt.Sprintf("%s/../", cwd)))

	buildDir, err := os.Getwd()

	version, err := loadVersion(fmt.Sprintf("%s/%s", buildDir, helmFolder))
	check(err)

	err = validateVersion(dockerEnv, binName, version)
	// TODO switch back on
	// if err != nil {
	// 	check(fmt.Errorf(
	// 		"%s\nThe app version is defined in \"%s/%s/Chart.yaml\" in the Version field",
	// 		err, buildDir, helmFolder))
	// }

	check(build(buildDir))
	fmt.Println("")

	check(dockerBuild(dockerEnv, version))
	fmt.Println("")

	h := helm.New(kubeconfig, buildDir, namespace, binName)

	check(h.Deploy())
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

	out, err := process.RunEnvRes(
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
	process.LogInfo("BUILD", fmt.Sprintf("building go app in %s", buildDir))

	// Build a statically linked binary that can live in a scratch container
	err := process.RunEnv(
		"BUILD",
		map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
		[]string{"go", "build", "-a", "-installsuffix", "cgo", "-o", binName, "."},
	)
	if err != nil {
		return err
	}
	process.LogInfo("BUILD", "success")

	err = process.RunEnv(
		"MOVE",
		map[string]string{},
		[]string{"mv", binName, "./app"},
	)
	if err != nil {
		return err
	}
	return nil
}

func dockerBuild(d docker, version string) error {
	return process.RunEnv(
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

func runTests(buildDir, minikubeIP string) error {
	err := process.RunEnv(
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

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
