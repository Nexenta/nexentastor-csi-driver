package remote

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// default wait timeout
	defaultWaitTimeout = 60 * time.Second

	// default wait interval
	defaultWaitInterval = 2 * time.Second
)

// Client - wrapper to run bash commands over ssh
type Client struct {
	// ConnectionString - user@host for ssh command
	ConnectionString string

	// WaitInterval - run command every N seconds to check the output
	WaitInterval time.Duration

	// WaitTimeout - consider command to fail after this timeout exceeded
	WaitTimeout time.Duration

	log *logrus.Entry
}

func (c *Client) String() string {
	return c.ConnectionString
}

// Exec - run command over ssh
func (c *Client) Exec(cmd string) (string, error) {
	l := c.log.WithField("func", "Exec()")
	l.Info(cmd)

	out, err := exec.Command("ssh", c.ConnectionString, cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Command 'ssh %v, %v' error: %v; out: %s", c.ConnectionString, cmd, err, out)
	}
	return fmt.Sprintf("%s", out), nil
}

// ExecAndWaitRegExp - wait command output to to satisfy regex or return error on timeout
func (c *Client) ExecAndWaitRegExp(cmd string, re *regexp.Regexp, inverted bool) error {
	l := c.log.WithField("func", "ExecAndWaitRegExp()")
	l.Infof("%v # wait: '%v'", cmd, re)

	done := make(chan error)
	timer := time.NewTimer(0)
	timeout := time.After(c.WaitTimeout)
	lastOutput := ""

	go func() {
		startTime := time.Now()
		for {
			select {
			case <-timer.C:
				out, err := c.Exec(cmd)
				if err != nil {
					done <- err
					return
				} else if (!inverted && re.MatchString(out)) || (inverted && !re.MatchString(out)) {
					done <- nil
					return
				}

				lastOutput = out
				waitingTimeSeconds := time.Since(startTime).Seconds()
				if waitingTimeSeconds >= c.WaitInterval.Seconds() {
					msg := fmt.Sprintf("waiting for %.0fs...", waitingTimeSeconds)
					if waitingTimeSeconds >= c.WaitTimeout.Seconds()/2 {
						l.Warn(msg)
					} else {
						l.Infof(msg)
					}
				}
				timer = time.NewTimer(c.WaitInterval)
			case <-timeout:
				timer.Stop()
				done <- fmt.Errorf(
					"Checking cmd output timeout exceeded (%v), "+
						"cmd: '%v', regexp: '%v', inverted: %v, last output:\n"+
						"---\n%v\n---\n",
					c.WaitTimeout,
					cmd,
					re,
					inverted,
					lastOutput,
				)
				return
			}
		}
	}()

	return <-done
}

// CopyFiles - copy local files to remote server
func (c *Client) CopyFiles(from, to string) error {
	l := c.log.WithField("func", "CopyFiles()")

	toAddress := fmt.Sprintf("%v:%v", c.ConnectionString, to)

	l.Infof("scp %v %v\n", from, toAddress)

	if out, err := exec.Command("scp", from, toAddress).CombinedOutput(); err != nil {
		return fmt.Errorf("Command 'scp %v %v' error: %v; out: %s", from, toAddress, err, out)
	}

	return nil
}

// NewClient - create new SSH remote client
func NewClient(connectionString string, log *logrus.Entry) (*Client, error) {
	l := log.WithFields(logrus.Fields{
		"address": connectionString,
		"cmp":     "remote",
	})

	client := &Client{
		ConnectionString: connectionString,
		WaitInterval:     defaultWaitInterval,
		WaitTimeout:      defaultWaitTimeout,
		log:              l,
	}

	_, err := client.Exec("date")
	if err != nil {
		return nil, fmt.Errorf("Failed to validate %v connection: %v", connectionString, err)
	}

	return client, nil
}
