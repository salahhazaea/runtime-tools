package validation

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mrunalp/fileutils"
	rspecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/specerror"
)

var (
	runtimeCommand = "runc"
)

func init() {
	runtimeInEnv := os.Getenv("RUNTIME")
	if runtimeInEnv != "" {
		runtimeCommand = runtimeInEnv
	}
}

func prepareBundle() (string, error) {
	// Setup a temporary test directory
	bundleDir, err := ioutil.TempDir("", "ocitest")
	if err != nil {
		return "", err
	}

	// Untar the root fs
	untarCmd := exec.Command("tar", "-xf", fmt.Sprintf("../rootfs-%s.tar.gz", runtime.GOARCH), "-C", bundleDir)
	_, err = untarCmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(bundleDir)
		return "", err
	}

	return bundleDir, nil
}

func getDefaultGenerator() *generate.Generator {
	g := generate.New()
	g.SetRootPath(".")
	g.SetProcessArgs([]string{"/runtimetest"})
	return &g
}

func runtimeInsideValidate(g *generate.Generator) error {
	bundleDir, err := prepareBundle()
	if err != nil {
		return err
	}
	r, err := NewRuntime(runtimeCommand, bundleDir)
	if err != nil {
		os.RemoveAll(bundleDir)
		return err
	}
	defer r.Clean(true)
	err = r.SetConfig(g)
	if err != nil {
		return err
	}
	err = fileutils.CopyFile("../runtimetest", filepath.Join(r.BundleDir, "runtimetest"))
	if err != nil {
		return err
	}

	r.SetID(uuid.NewV4().String())
	err = r.Create()
	if err != nil {
		return err
	}
	return r.Start()
}

func TestValidateBasic(t *testing.T) {
	g := getDefaultGenerator()

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test whether rootfs Readonly can be applied as false
func TestValidateRootFSReadWrite(t *testing.T) {
	g := getDefaultGenerator()
	g.SetRootReadonly(false)

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test whether rootfs Readonly can be applied as true
func TestValidateRootFSReadonly(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skip this test on windows platform")
	}

	g := getDefaultGenerator()
	g.SetRootReadonly(true)

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test whether hostname can be applied or not
func TestValidateHostname(t *testing.T) {
	g := getDefaultGenerator()
	g.SetHostname("hostname-specific")

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test whether mounts are correctly mounted
func TestValidateMounts(t *testing.T) {
	// TODO mounts generation options have not been implemented
	// will add it after 'mounts generate' done
}

// Test whether rlimits can be applied or not
func TestValidateRlimits(t *testing.T) {
	g := getDefaultGenerator()
	g.AddProcessRlimits("RLIMIT_NOFILE", 1024, 1024)

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test whether sysctls can be applied or not
func TestValidateSysctls(t *testing.T) {
	g := getDefaultGenerator()
	g.AddLinuxSysctl("net.ipv4.ip_forward", "1")

	assert.Nil(t, runtimeInsideValidate(g))
}

// Test Create operation
func TestValidateCreate(t *testing.T) {
	g := generate.New()
	g.SetRootPath(".")
	g.SetProcessArgs([]string{"ls"})

	bundleDir, err := prepareBundle()
	assert.Nil(t, err)

	r, err := NewRuntime(runtimeCommand, bundleDir)
	assert.Nil(t, err)
	defer r.Clean(true)

	err = r.SetConfig(&g)
	assert.Nil(t, err)

	containerID := uuid.NewV4().String()
	cases := []struct {
		id          string
		errExpected bool
		err         error
	}{
		{"", false, specerror.NewError(specerror.CreateWithBundlePathAndID, fmt.Errorf("create MUST generate an error if the ID is not provided"), rspecs.Version)},
		{containerID, true, specerror.NewError(specerror.CreateNewContainer, fmt.Errorf("create MUST create a new container"), rspecs.Version)},
		{containerID, false, specerror.NewError(specerror.CreateWithUniqueID, fmt.Errorf("create MUST generate an error if the ID provided is not unique"), rspecs.Version)},
	}

	for _, c := range cases {
		r.SetID(c.id)
		err := r.Create()
		assert.Equal(t, c.errExpected, err == nil, c.err.Error())

		if err == nil {
			state, _ := r.State()
			assert.Equal(t, c.id, state.ID, c.err.Error())
		}
	}
}
