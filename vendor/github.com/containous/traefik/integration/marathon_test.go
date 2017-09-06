package integration

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/types"
	marathon "github.com/gambol99/go-marathon"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

const (
	containerNameMesosSlave = "mesos-slave"
	containerNameMarathon   = "marathon"
)

// Marathon test suites (using libcompose)
type MarathonSuite struct {
	BaseSuite
	marathonURL string
}

func (s *MarathonSuite) SetUpSuite(c *check.C) {
	s.createComposeProject(c, "marathon")
	s.composeProject.Start(c)

	marathonIPAddr := s.composeProject.Container(c, containerNameMarathon).NetworkSettings.IPAddress
	c.Assert(marathonIPAddr, checker.Not(checker.HasLen), 0)
	s.marathonURL = "http://" + marathonIPAddr + ":8080"

	// Wait for Marathon readiness prior to creating the client so that we
	// don't run into the "all cluster members down" state right from the
	// start.
	err := try.GetRequest(s.marathonURL+"/v2/leader", 1*time.Minute, try.StatusCodeIs(http.StatusOK))
	c.Assert(err, checker.IsNil)

	// Add entry for Mesos slave container IP address in the hosts file so
	// that Traefik can properly forward traffic.
	// This is necessary as long as we are still using the docker-compose v1
	// spec. Once we switch to v2 or higher, we can have both the test/builder
	// container and the Mesos slave container join the same custom network and
	// enjoy DNS-discoverable container host names.
	mesosSlaveIPAddr := s.composeProject.Container(c, containerNameMesosSlave).NetworkSettings.IPAddress
	c.Assert(mesosSlaveIPAddr, checker.Not(checker.HasLen), 0)
	err = s.extendDockerHostsFile(containerNameMesosSlave, mesosSlaveIPAddr)
	c.Assert(err, checker.IsNil)
}

// extendDockerHostsFile extends the hosts file (/etc/hosts) by the given
// host/IP address mapping if we are running inside a container.
func (s *MarathonSuite) extendDockerHostsFile(host, ipAddr string) error {
	const hostsFile = "/etc/hosts"

	// Determine if the run inside a container. The most reliable way to
	// do this is to inject an indicator, which we do in terms of an
	// environment variable.
	// (See also https://groups.google.com/d/topic/docker-user/JOGE7AnJ3Gw/discussion.)
	if os.Getenv("CONTAINER") == "DOCKER" {
		// We are running inside a container -- extend the hosts file.
		file, err := os.OpenFile(hostsFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err = file.WriteString(fmt.Sprintf("%s\t%s\n", ipAddr, host)); err != nil {
			return err
		}
	}

	return nil
}

func deployApplication(c *check.C, client marathon.Marathon, application *marathon.Application) {
	deploy, err := client.UpdateApplication(application, false)
	c.Assert(err, checker.IsNil)
	// Wait for deployment to complete.
	c.Assert(client.WaitOnDeployment(deploy.DeploymentID, 1*time.Minute), checker.IsNil)
}

func (s *MarathonSuite) TestConfigurationUpdate(c *check.C) {
	// Start Traefik.
	file := s.adaptFile(c, "fixtures/marathon/simple.toml", struct {
		MarathonURL string
	}{s.marathonURL})
	defer os.Remove(file)
	cmd, output := s.cmdTraefik(withConfigFile(file))
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Wait for Traefik to turn ready.
	err = try.GetRequest("http://127.0.0.1:8000/", 2*time.Second, try.StatusCodeIs(http.StatusNotFound))
	c.Assert(err, checker.IsNil)

	// Prepare Marathon client.
	config := marathon.NewDefaultConfig()
	config.URL = s.marathonURL
	client, err := marathon.NewClient(config)
	c.Assert(err, checker.IsNil)

	// Show the Traefik log if any assertion fails. If the entire test runs
	// to a successful completion, we flip the flag at the very end and don't
	// display anything.
	showTraefikLog := true
	defer func() {
		if showTraefikLog {
			s.displayTraefikLog(c, output)
		}
	}()

	// Create test application to be deployed.
	app := marathon.NewDockerApplication().
		Name("/whoami").
		CPU(0.1).
		Memory(32).
		AddLabel(types.LabelFrontendRule, "PathPrefix:/service")
	app.Container.Docker.Bridged().
		Expose(80).
		Container("emilevauge/whoami")

	// Deploy the test application.
	deployApplication(c, client, app)

	// Query application via Traefik.
	err = try.GetRequest("http://127.0.0.1:8000/service", 30*time.Second, try.StatusCodeIs(http.StatusOK))
	c.Assert(err, checker.IsNil)

	// Create test application with services to be deployed.
	app = marathon.NewDockerApplication().
		Name("/whoami").
		CPU(0.1).
		Memory(32).
		AddLabel(types.ServiceLabel(types.LabelFrontendRule, "app"), "PathPrefix:/app")
	app.Container.Docker.Bridged().
		Expose(80).
		Container("emilevauge/whoami")

	// Deploy the test application.
	deployApplication(c, client, app)

	// Query application via Traefik.
	err = try.GetRequest("http://127.0.0.1:8000/app", 30*time.Second, try.StatusCodeIs(http.StatusOK))
	c.Assert(err, checker.IsNil)

	showTraefikLog = false
}
