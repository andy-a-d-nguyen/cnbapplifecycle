package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/cnbapplifecycle/pkg/staging"
	"github.com/docker/docker/api/types/container"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
)

func runInContainer(ctx context.Context, container testcontainers.Container, cmd ...string) error {
	code, _, err := container.Exec(ctx, cmd)
	if code != 0 {
		return fmt.Errorf("failed to run %q, RC: %d", cmd[0], code)
	}
	if err != nil {
		return err
	}
	return nil
}

var _ = Describe("Lifecycle", func() {
	var (
		testContainer testcontainers.Container
		err           error
		cacheDir      string
	)

	BeforeEach(func() {
		cacheDir = GinkgoT().TempDir()
		Expect(err).ToNot(HaveOccurred())
		req := testcontainers.ContainerRequest{
			Image:         "ubuntu:noble",
			ImagePlatform: "linux/amd64",
			ExposedPorts:  []string{"8080/tcp"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.Binds = []string{cacheDir + ":/tmp/cache"}
			},
			ConfigModifier: func(c *container.Config) {
				c.Tty = true
			},
			Env: map[string]string{
				"CNB_LAYERS_DIR": "/home/ubuntu/layers",
				"CNB_APP_DIR":    "/home/ubuntu/workspace",
				"CNB_STACK_ID":   "cflinuxfs4",
				"CNB_USER_ID":    "1000",
				"CNB_GROUP_ID":   "1000",
				"CNB_LOG_LEVEL":  "DEBUG",
				"CNB_NO_COLOR":   "true",
				"BP_JVM_VERSION": "17",
			},
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{PostStarts: append(defaultContainerSetup(), prepareContainerWorkspace("./testdata/workspace/", "java"))}},
		}
		testContainer, err = testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		Expect(err).To(BeNil())
	})

	It("should build an app using system buildpacks", func() {
		code, out, err := testContainer.Exec(context.Background(), []string{
			"/tmp/builder",
			"-b", "gcr.io/paketo-buildpacks/java",
			"-r", "/tmp/build-result.json",
			"-d", "/tmp/droplet.tgz",
			"-l", "/home/ubuntu/layers",
			"-w", "/home/ubuntu/workspace",
			"--pass-env-var", "BP_JVM_VERSION",
		}, exec.WithUser("ubuntu"))
		Expect(err).To(BeNil())
		assertExitCode(code, 0, out)

		buf := bytes.NewBufferString("")
		_, err = io.Copy(buf, out)
		Expect(err).To(BeNil())
		outString := buf.String()
		Expect(outString).To(ContainSubstring("Using buildpacks: file://"))
		Expect(outString).To(ContainSubstring("Run image info in analyzed metadata is"))
		Expect(outString).To(ContainSubstring("Checking for match against descriptor"))
		Expect(outString).To(ContainSubstring("Finished running build"))
		Expect(outString).To(ContainSubstring("Copying SBOM files"))
		Expect(outString).To(ContainSubstring("Listing processes"))
		Expect(outString).To(ContainSubstring("Builder ran for"))
		Expect(outString).To(ContainSubstring("result file saved to"))
		Expect(outString).To(ContainSubstring("droplet archive saved to"))

		r, err := testContainer.CopyFileFromContainer(context.Background(), "/tmp/build-result.json")
		Expect(err).To(BeNil())
		defer r.Close()

		resultBuf := bytes.NewBuffer(nil)
		_, err = io.Copy(resultBuf, r)
		Expect(err).To(BeNil())

		result := staging.StagingResult{}
		Expect(json.Unmarshal(resultBuf.Bytes(), &result)).To(Succeed())
		Expect(result.LifecycleType).To(Equal("cnb"))
		Expect(result.ProcessTypes).To(Equal(staging.ProcessTypes{
			"executable-jar": "java org.springframework.boot.loader.JarLauncher",
			"task":           "java org.springframework.boot.loader.JarLauncher",
			"web":            "java org.springframework.boot.loader.JarLauncher",
		}))
		Expect(result.Buildpacks).To(HaveLen(10))
	})

	It("build an app using custom buildpacks", func() {
		By("building the app", func() {
			code, out, err := testContainer.Exec(context.Background(), []string{
				"/tmp/builder",
				"-b", "gcr.io/paketo-buildpacks/java",
				"-r", "/tmp/build-result.json",
				"-d", "/tmp/droplet.tgz",
				"-l", "/home/ubuntu/layers",
				"-w", "/home/ubuntu/workspace",
				"--pass-env-var", "BP_JVM_VERSION",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			buf := bytes.NewBufferString("")
			_, err = io.Copy(buf, out)
			Expect(err).To(BeNil())

			outString := buf.String()
			Expect(outString).To(ContainSubstring("Run image info in analyzed metadata is"))
			Expect(outString).To(ContainSubstring("Checking for match against descriptor"))
			Expect(outString).To(ContainSubstring("Finished running build"))
			Expect(outString).To(ContainSubstring("Copying SBOM files"))
			Expect(outString).To(ContainSubstring("Listing processes"))
			Expect(outString).To(ContainSubstring("Builder ran for"))
			Expect(outString).To(ContainSubstring("result file saved to"))
			Expect(outString).To(ContainSubstring("droplet archive saved to"))

			r, err := testContainer.CopyFileFromContainer(context.Background(), "/tmp/build-result.json")
			Expect(err).To(BeNil())
			defer r.Close()

			resultBuf := bytes.NewBuffer(nil)
			_, err = io.Copy(resultBuf, r)
			Expect(err).To(BeNil())

			result := staging.StagingResult{}
			Expect(json.Unmarshal(resultBuf.Bytes(), &result)).To(Succeed())
			Expect(result.LifecycleType).To(Equal("cnb"))
			Expect(result.ProcessTypes).To(Equal(staging.ProcessTypes{
				"executable-jar": "java org.springframework.boot.loader.JarLauncher",
				"task":           "java org.springframework.boot.loader.JarLauncher",
				"web":            "java org.springframework.boot.loader.JarLauncher",
			}))
			Expect(result.Buildpacks).To(HaveLen(10))
		})

		By("building it again with cache", func() {
			code, out, err := testContainer.Exec(context.Background(), []string{
				"rm", "-rf", "/home/ubuntu/layers",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			code, out, err = testContainer.Exec(context.Background(), []string{
				"rm", "-rf", "/home/ubuntu/workspace",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			code, out, err = testContainer.Exec(context.Background(), []string{
				"rm", "-rf", "/tmp/buildpacks",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			Expect(testContainer.CopyDirToContainer(context.Background(), "testdata/workspace", "/home/ubuntu/", 0o755)).To(Succeed())
			code, out, err = testContainer.Exec(context.Background(), []string{"chown", "-R", "ubuntu:ubuntu", "/home/ubuntu/workspace"})
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			code, out, err = testContainer.Exec(context.Background(), []string{
				"/tmp/builder",
				"-b", "gcr.io/paketo-buildpacks/java",
				"-r", "/tmp/build-result.json",
				"-d", "/tmp/droplet.tgz",
				"-l", "/home/ubuntu/layers",
				"-w", "/home/ubuntu/workspace",
				"--pass-env-var", "BP_JVM_VERSION",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			buf := new(strings.Builder)
			_, err = io.Copy(buf, out)
			Expect(err).To(BeNil())

			outString := buf.String()
			Expect(outString).To(ContainSubstring("Restoring metadata for"))
			Expect(outString).To(ContainSubstring("Restoring data for"))
			Expect(outString).To(ContainSubstring("previously cached download"))
			Expect(outString).To(ContainSubstring("Finished running build"))
			Expect(outString).To(ContainSubstring("Copying SBOM files"))
			Expect(outString).To(ContainSubstring("Listing processes"))
			Expect(outString).To(ContainSubstring("Builder ran for"))
			Expect(outString).To(ContainSubstring("result file saved to"))
			Expect(outString).To(ContainSubstring("droplet archive saved to"))
		})

		By("running a task", func() {
			buf := bytes.NewBufferString("")
			_, out, err := testContainer.Exec(context.Background(), []string{"/tmp/launcher", "--", "java", "--version"}, exec.WithUser("ubuntu"))

			_, copyErr := io.Copy(buf, out)
			Expect(copyErr).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("build 17"))
			Expect(err).To(BeNil())
		})

		By("launching the app", func() {
			go func() {
				defer GinkgoRecover()

				buf := bytes.NewBufferString("")
				_, out, err := testContainer.Exec(context.Background(), []string{"/tmp/launcher"}, exec.WithUser("ubuntu"))
				_, copyErr := io.Copy(buf, out)
				Expect(copyErr).To(BeNil())
				Expect(buf.String()).To(ContainSubstring("Started DemoApplication in"))
				Expect(err).To(BeNil())
			}()

			port, err := testContainer.MappedPort(context.Background(), "8080")
			Expect(err).To(BeNil())
			Eventually(func() string {
				return fetch(fmt.Sprintf("http://127.0.0.1:%s", port.Port()))
			}).WithTimeout(10 * time.Second).Should(Equal("Greetings from Spring Boot!"))
		})

		By("running a sidecar", func() {
			Expect(testContainer.CopyFileToContainer(context.Background(), "testdata/workspace/sidecar.sh", "/home/ubuntu/workspace/sidecar.sh", 0o755)).To(Succeed())
			buf := bytes.NewBufferString("")
			_, out, err := testContainer.Exec(context.Background(), []string{"/tmp/launcher", "app", "./sidecar.sh"}, exec.WithUser("ubuntu"))

			_, copyErr := io.Copy(buf, out)
			Expect(copyErr).To(BeNil())
			Expect(buf.String()).To(ContainSubstring("sidecar is running"))
			Expect(err).To(BeNil())
		})
	})

	AfterEach(func() {
		Expect(testContainer.Terminate(context.Background())).To(Succeed())
	})
})

var _ = Describe("Ensuring web process", func() {
	var (
		testContainer testcontainers.Container
		err           error
		cacheDir      string
	)

	BeforeEach(func() {
		cacheDir = GinkgoT().TempDir()
		Expect(err).ToNot(HaveOccurred())
		req := testcontainers.ContainerRequest{
			Image:         "ubuntu:noble",
			ImagePlatform: "linux/amd64",
			ExposedPorts:  []string{"8080/tcp"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.Binds = []string{cacheDir + ":/tmp/cache"}
			},
			ConfigModifier: func(c *container.Config) {
				c.Tty = true
			},
			Env: map[string]string{
				"CNB_LAYERS_DIR": "/home/ubuntu/layers",
				"CNB_APP_DIR":    "/home/ubuntu/workspace_go",
				"CNB_STACK_ID":   "cflinuxfs4",
				"CNB_USER_ID":    "1000",
				"CNB_GROUP_ID":   "1000",
				"CNB_LOG_LEVEL":  "DEBUG",
				"CNB_NO_COLOR":   "true",
			},
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{PostStarts: append(defaultContainerSetup(), prepareContainerWorkspace("./testdata/workspace_go", "go"))}},
		}
		testContainer, err = testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		Expect(err).To(BeNil())
	})

	It("web process is added even when buildpack doesn't add one", func() {
		By("building the app", func() {
			code, out, err := testContainer.Exec(context.Background(), []string{
				"/tmp/builder",
				"-b", "gcr.io/paketo-buildpacks/go",
				"-r", "/tmp/build-result.json",
				"-d", "/tmp/droplet.tgz",
				"-l", "/home/ubuntu/layers",
				"-w", "/home/ubuntu/workspace_go",
			}, exec.WithUser("ubuntu"))
			Expect(err).To(BeNil())
			assertExitCode(code, 0, out)

			r, err := testContainer.CopyFileFromContainer(context.Background(), "/tmp/build-result.json")
			Expect(err).To(BeNil())
			defer r.Close()

			resultBuf := bytes.NewBuffer(nil)
			_, err = io.Copy(resultBuf, r)
			Expect(err).To(BeNil())

			result := staging.StagingResult{}
			Expect(json.Unmarshal(resultBuf.Bytes(), &result)).To(Succeed())
			Expect(result.LifecycleType).To(Equal("cnb"))
			Expect(result.ProcessTypes).To(Equal(staging.ProcessTypes{
				"hello-world": "/home/ubuntu/layers/paketo-buildpacks_go-build/targets/bin/hello-world",
				"web":         "/home/ubuntu/layers/paketo-buildpacks_go-build/targets/bin/hello-world",
			}))
		})

	})
	AfterEach(func() {
		Expect(testContainer.Terminate(context.Background())).To(Succeed())
	})
})

func fetch(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	buf := bytes.NewBufferString("")
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return ""
	}

	return buf.String()
}

func defaultContainerSetup() []testcontainers.ContainerHook {
	return []testcontainers.ContainerHook{
		func(ctx context.Context, container testcontainers.Container) error {
			code, _, err := container.Exec(ctx, []string{"apt", "update"})
			if code != 0 {
				return fmt.Errorf("failed to run apt update, RC: %d", code)
			}

			return err
		},
		func(ctx context.Context, container testcontainers.Container) error {
			code, _, err := container.Exec(ctx, []string{"apt", "install", "ca-certificates", "skopeo", "-y"})
			if code != 0 {
				return fmt.Errorf("failed to run install ca-certificates, RC: %d", code)
			}

			return err
		},
	}
}

func prepareContainerWorkspace(workspaceDir string, buildpack string) func(context.Context, testcontainers.Container) error {

	return func(ctx context.Context, container testcontainers.Container) error {
		if err := container.CopyFileToContainer(ctx, "../bin/builder", "/tmp/builder", 0o755); err != nil {
			return err
		}

		if err := container.CopyFileToContainer(ctx, "../bin/launcher", "/tmp/launcher", 0o755); err != nil {
			return err
		}

		if err := container.CopyDirToContainer(ctx, workspaceDir, "/home/ubuntu/", 0o755); err != nil {
			return err
		}

		if err := runInContainer(ctx, container, "chown", "-R", "ubuntu:ubuntu", filepath.Join("/home/ubuntu", filepath.Base(workspaceDir))); err != nil {
			return err
		}

		if err := runInContainer(ctx, container, "mkdir", "-p", "/tmp/buildpacks"); err != nil {
			return err
		}

		if err := runInContainer(ctx, container, "skopeo", "copy", fmt.Sprintf("docker://gcr.io/paketo-buildpacks/%s:latest", buildpack), "oci:/tmp/buildpacks/10bfa3ba0b8af13e"); err != nil {
			return err
		}

		if err := runInContainer(ctx, container, "chown", "-R", "ubuntu:ubuntu", "/tmp/buildpacks"); err != nil {
			return err
		}

		return nil
	}
}

func assertExitCode(code int, expected int, out io.Reader) {
	GinkgoHelper()
	if code != expected {
		buf := bytes.NewBufferString("")
		_, _ = io.Copy(buf, out)
		fmt.Print(buf.String())
		Fail(fmt.Sprintf("expected code to equal %d, but was %d", expected, code))
	}
}
