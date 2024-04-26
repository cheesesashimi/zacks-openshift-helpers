package legacycmds

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/utils"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/openshift/machine-config-operator/pkg/controller/build"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/framework"
	"github.com/spf13/cobra"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	renderOpts struct {
		poolName             string
		includeMachineConfig bool
		targetDir            string
	}
)

func RenderCommand() *cobra.Command {
	renderCmd := &cobra.Command{
		Use:   "render",
		Short: "Renders the on-cluster build Dockerfile to disk",
		Long:  "",
		RunE:  runRenderCmd,
	}

	renderCmd.PersistentFlags().StringVar(&renderOpts.poolName, "pool", DefaultLayeredPoolName, "Pool name to render")
	renderCmd.PersistentFlags().StringVar(&renderOpts.targetDir, "dir", "", "Dir to store rendered Dockerfile and MachineConfig in")

	return renderCmd
}

func runRenderCmd(_ *cobra.Command, _ []string) error {
	utils.ParseFlags()

	if renderOpts.poolName == "" {
		return fmt.Errorf("no pool name provided")
	}

	cs := framework.NewClientSet("")

	targetDir, err := GetDir(renderOpts.targetDir)
	if err != nil {
		return err
	}

	dir := filepath.Join(targetDir, renderOpts.poolName)

	if err := renderDockerfileToDisk(cs, renderOpts.poolName, dir); err != nil {
		return err
	}

	mcp, err := cs.MachineConfigPools().Get(context.TODO(), renderOpts.poolName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	return storeMachineConfigOnDisk(cs, mcp, dir)
}

func renderDockerfile(ibr *build.ImageBuildRequest, out io.Writer, copyToStdout bool) error {
	dockerfileTemplate, err := fetchDockerfileTemplate()
	if err != nil {
		return fmt.Errorf("unable to fetch Dockerfile template: %w", err)
	}

	tmpl, err := template.New("dockerfile").Parse(string(dockerfileTemplate))
	if err != nil {
		return err
	}

	if copyToStdout {
		out = io.MultiWriter(out, os.Stdout)
	}

	return tmpl.Execute(out, ibr)
}

func renderDockerfileToDisk(cs *framework.ClientSet, targetPool, dir string) error {
	ibr, err := getImageBuildRequest(cs, targetPool)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	dockerfilePath := filepath.Join(dir, "Dockerfile")

	dockerfile, err := os.Create(dockerfilePath)
	defer func() {
		if err := dockerfile.Close(); err != nil {
			panic(err)
		}
	}()

	if err != nil {
		return err
	}

	klog.Infof("Rendered Dockerfile to disk at %s", dockerfilePath)
	return renderDockerfile(ibr, dockerfile, false)
}

func storeMachineConfigOnDisk(cs *framework.ClientSet, pool *mcfgv1.MachineConfigPool, dir string) error {
	mc, err := cs.MachineConfigs().Get(context.TODO(), pool.Spec.Configuration.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	out, err := json.Marshal(mc)
	if err != nil {
		return err
	}

	if err := os.Mkdir(filepath.Join(dir, "machineconfig"), 0o755); err != nil {
		return err
	}

	compressed, err := compressAndEncode(out)
	if err != nil {
		return err
	}

	mcPath := filepath.Join(dir, "machineconfig", "machineconfig.json.gz")

	if err := os.WriteFile(mcPath, compressed.Bytes(), 0o755); err != nil {
		return err
	}

	klog.Infof("Stored MachineConfig %s on disk at %s", mc.Name, mcPath)
	return nil
}

// Compresses and base-64 encodes a given byte array. Ideal for loading an
// arbitrary byte array into a ConfigMap or Secret.
func compressAndEncode(payload []byte) (*bytes.Buffer, error) {
	out := bytes.NewBuffer(nil)

	if len(payload) == 0 {
		return out, nil
	}

	// We need to base64-encode our gzipped data so we can marshal it in and out
	// of a string since ConfigMaps and Secrets expect a textual representation.
	base64Enc := base64.NewEncoder(base64.StdEncoding, out)
	defer base64Enc.Close()

	err := compress(bytes.NewBuffer(payload), base64Enc)
	if err != nil {
		return nil, fmt.Errorf("could not compress and encode payload: %w", err)
	}

	err = base64Enc.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close base64 encoder: %w", err)
	}

	return out, err
}

// Compresses a given io.Reader to a given io.Writer
func compress(r io.Reader, w io.Writer) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("could not initialize gzip writer: %w", err)
	}

	defer gz.Close()

	if _, err := io.Copy(gz, r); err != nil {
		return fmt.Errorf("could not compress payload: %w", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("could not close gzipwriter: %w", err)
	}

	return nil
}

func fetchDockerfileTemplate() ([]byte, error) {
	dockerfileTemplate, err := fetchDockerfileTemplateFromDisk()
	if err == nil {
		return dockerfileTemplate, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("could not get Dockerfile template from disk: %w", err)
	}

	klog.Infof("Could not fetch Dockerfile template from disk, falling back to GitHub...")
	dockerfileTemplate, err = fetchDockerfileTemplateFromGitHub()
	if err != nil {
		return nil, fmt.Errorf("could not fetch Dockerfile template from GitHub: %w", err)
	}

	return dockerfileTemplate, nil
}

// TODO: Export the template from the assets package or figure out how to build it into this binary.
func fetchDockerfileTemplateFromDisk() ([]byte, error) {
	templatePath := "/Users/zzlotnik/go/src/github.com/openshift/machine-config-operator/pkg/controller/build/assets/Dockerfile.on-cluster-build-template"

	klog.Infof("Attempting to fetch Dockerfile template from %q", templatePath)

	dockerfileTemplate, err := os.ReadFile(templatePath)
	if err != nil {
		klog.Errorf("Attempt failed: %s", err)
		return nil, err
	}

	klog.Infof("Attempt succeeded")
	return dockerfileTemplate, nil
}

func fetchDockerfileTemplateFromGitHub() ([]byte, error) {
	templateURL := "https://raw.githubusercontent.com/openshift/machine-config-operator/master/pkg/controller/build/assets/Dockerfile.on-cluster-build-template"

	klog.Infof("Attempting to fetch Dockerfile template from %q", templateURL)

	resp, err := http.Get(templateURL)
	if err != nil {
		klog.Errorf("Attempt unsuccessful: %s", err)
		return nil, fmt.Errorf("could not fetch %q from %q: %w", filepath.Base(templateURL), templateURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not fetch %q from %q: %w", filepath.Base(templateURL), templateURL, fmt.Errorf("got HTTP status %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
	}

	buf := bytes.NewBuffer([]byte{})
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	if _, err := io.Copy(buf, resp.Body); err != nil {
		return nil, err
	}

	klog.Infof("Attempt successful")
	return buf.Bytes(), nil
}

// TODO: Dedupe this with the code from the buildcontroller package.
func getImageBuildRequest(cs *framework.ClientSet, targetPool string) (*build.ImageBuildRequest, error) {
	osImageURLConfigMap, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Get(context.TODO(), "machine-config-osimageurl", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	customDockerfile, err := cs.CoreV1Interface.ConfigMaps(ctrlcommon.MCONamespace).Get(context.TODO(), "on-cluster-build-custom-dockerfile", metav1.GetOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return nil, err
	}

	var customDockerfileContents string
	if customDockerfile != nil {
		customDockerfileContents = customDockerfile.Data[targetPool]
	}

	mcp, err := cs.MachineConfigPools().Get(context.TODO(), targetPool, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	buildReq := build.ImageBuildRequest{
		Pool: mcp,
		BaseImage: build.ImageInfo{
			Pullspec: osImageURLConfigMap.Data["baseOSContainerImage"],
		},
		ExtensionsImage: build.ImageInfo{
			Pullspec: osImageURLConfigMap.Data["baseOSExtensionsContainerImage"],
		},
		ReleaseVersion:   osImageURLConfigMap.Data["releaseVersion"],
		CustomDockerfile: customDockerfileContents,
	}

	return &buildReq, nil
}
