package builders

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	buildv1 "github.com/openshift/api/build/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	aggerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"

	"github.com/openshift/machine-config-operator/test/framework"
)

const (
	buildName string = "mco-image-build"
)

type OpenshiftBuilderOpts struct {
	ImageStreamName     string
	ImageStreamPullspec string
	Dockerfile          string
	BranchName          string
	RemoteURL           string
	FollowBuild         bool
}

type OpenshiftBuilder struct {
	opts OpenshiftBuilderOpts
	cs   *framework.ClientSet
}

func NewOpenshiftBuilder(cs *framework.ClientSet, opts OpenshiftBuilderOpts) *OpenshiftBuilder {
	return &OpenshiftBuilder{
		opts: opts,
		cs:   cs,
	}
}

func (o *OpenshiftBuilder) Build() error {
	_, err := o.cs.BuildV1Interface.Builds(ctrlcommon.MCONamespace).Create(context.TODO(), o.prepareBuild(), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create build: %w", err)
	}

	klog.Infof("Build %s created, waiting for completion...", buildName)

	if err := o.waitForBuildToComplete(); err != nil {
		return err
	}

	klog.Infof("Build %s completed. Cleaning up build object...", buildName)
	err = o.cs.BuildV1Interface.Builds(ctrlcommon.MCONamespace).Delete(context.TODO(), buildName, metav1.DeleteOptions{})

	return nil
}

func (o *OpenshiftBuilder) waitForBuildToComplete() error {
	name := fmt.Sprintf("build/%s", buildName)
	cmd := exec.Command("oc", "logs", "-f", name, "-n", ctrlcommon.MCONamespace)

	if o.opts.FollowBuild {
		klog.Infof("Streaming build logs...")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmdErr := cmd.Run()
	if cmdErr == nil {
		return nil
	}

	b, err := o.cs.BuildV1Interface.Builds(ctrlcommon.MCONamespace).Get(context.TODO(), buildName, metav1.GetOptions{})
	if err != nil {
		return aggerrs.NewAggregate([]error{
			err,
			cmdErr,
		})
	}

	if b.Status.Phase == buildv1.BuildPhaseComplete {
		return nil
	}

	runningStatuses := map[buildv1.BuildPhase]struct{}{
		buildv1.BuildPhaseNew:     {},
		buildv1.BuildPhasePending: {},
		buildv1.BuildPhaseRunning: {},
	}

	if _, ok := runningStatuses[b.Status.Phase]; ok {
		return o.waitForBuildToComplete()
	}

	return fmt.Errorf("build is in phase %s: %w", b.Status.Phase, cmdErr)
}

func (o *OpenshiftBuilder) prepareBuild() *buildv1.Build {
	skipLayers := buildv1.ImageOptimizationSkipLayers

	return &buildv1.Build{
		TypeMeta: metav1.TypeMeta{
			Kind: "Build",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctrlcommon.MCONamespace,
			Name:      buildName,
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type:       buildv1.BuildSourceDockerfile,
					Dockerfile: &o.opts.Dockerfile,
					Git: &buildv1.GitBuildSource{
						URI: o.opts.RemoteURL,
						Ref: o.opts.BranchName,
					},
				},
				Strategy: buildv1.BuildStrategy{
					DockerStrategy: &buildv1.DockerBuildStrategy{
						// Squashing layers is good as long as it doesn't cause problems with what
						// the users want to do. It says "some syntax is not supported"
						ImageOptimizationPolicy: &skipLayers,
					},
					Type: buildv1.DockerBuildStrategyType,
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Name: o.opts.ImageStreamName + ":latest",
						Kind: "ImageStreamTag",
					},
				},
			},
		},
	}
}
