package interface_tests

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var (
	// port = 8443
	port = 8080
	ip   = "192.168.0.11"
)

func TestGetTrees(t *testing.T) {
	envIP := os.Getenv("MINIKUBE_IP")
	if envIP != "" {
		ip = envIP
	}
	treePath := "tree"
	// 	fmt.Sprintf("curl localhost:%v/%s -H Host:local.ecosia.org",
	// 	port, treePath,
	// ),
	out, err := run(
		fmt.Sprintf("curl %s/%s -H Host:local.ecosia.org",
			ip, treePath,
		),
	)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	fmt.Printf("%s\n", out)
	if !strings.Contains(out, "{\"myFavouriteTree\":\"Sequoia\"}") {
		fmt.Printf("unexpected response %s\n", out)
		t.FailNow()
	}
}

func run(cmd string) (string, error) {
	fmt.Println(cmd)
	cmdSlice := strings.Split(cmd, " ")
	if len(cmdSlice) == 0 {
		return "", fmt.Errorf("command length is zero")
	}

	c := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	resp, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"command \"%s\" failed - error code: \"%s\", message: \"\n%s\n\"",
			cmd,
			err,
			string(resp),
		)
	}
	return string(resp), nil
}
