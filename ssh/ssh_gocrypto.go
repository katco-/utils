// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ssh

import (
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/utils"
	"golang.org/x/crypto/ssh"
)

const sshDefaultPort = 22

// GoCryptoClient is an implementation of Client that
// uses the embedded go.crypto/ssh SSH client.
//
// GoCryptoClient is intentionally limited in the
// functionality that it enables, as it is currently
// intended to be used only for non-interactive command
// execution.
type GoCryptoClient struct {
	signers []ssh.Signer
}

// NewGoCryptoClient creates a new GoCryptoClient.
//
// If no signers are specified, NewGoCryptoClient will
// use the private key generated by LoadClientKeys.
func NewGoCryptoClient(signers ...ssh.Signer) (*GoCryptoClient, error) {
	return &GoCryptoClient{signers: signers}, nil
}

// Command implements Client.Command.
func (c *GoCryptoClient) Command(host string, command []string, options *Options) *Cmd {
	shellCommand := utils.CommandString(command...)
	signers := c.signers
	if len(signers) == 0 {
		signers = privateKeys()
	}
	user, host := splitUserHost(host)
	port := sshDefaultPort
	var proxyCommand []string
	if options != nil {
		if options.port != 0 {
			port = options.port
		}
		proxyCommand = options.proxyCommand
	}
	logger.Tracef(`running (equivalent of): ssh "%s@%s" -p %d '%s'`, user, host, port, shellCommand)
	return &Cmd{impl: &goCryptoCommand{
		signers:      signers,
		user:         user,
		addr:         net.JoinHostPort(host, strconv.Itoa(port)),
		command:      shellCommand,
		proxyCommand: proxyCommand,
	}}
}

// Copy implements Client.Copy.
//
// Copy is currently unimplemented, and will always return an error.
func (c *GoCryptoClient) Copy(args []string, options *Options) error {
	return errors.Errorf("scp command is not implemented (OpenSSH scp not available in PATH)")
}

type goCryptoCommand struct {
	signers      []ssh.Signer
	user         string
	addr         string
	command      string
	proxyCommand []string
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	client       *ssh.Client
	sess         *ssh.Session
}

var sshDial = ssh.Dial

var sshDialWithProxy = func(addr string, proxyCommand []string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if len(proxyCommand) == 0 {
		return sshDial("tcp", addr, config)
	}
	// User has specified a proxy. Create a pipe and
	// redirect the proxy command's stdin/stdout to it.
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	for i, arg := range proxyCommand {
		arg = strings.Replace(arg, "%h", host, -1)
		if port != "" {
			arg = strings.Replace(arg, "%p", port, -1)
		}
		arg = strings.Replace(arg, "%r", config.User, -1)
		proxyCommand[i] = arg
	}
	client, server := net.Pipe()
	logger.Tracef(`executing proxy command %q`, proxyCommand)
	cmd := exec.Command(proxyCommand[0], proxyCommand[1:]...)
	cmd.Stdin = server
	cmd.Stdout = server
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(client, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(conn, chans, reqs), nil
}

func (c *goCryptoCommand) ensureSession() (*ssh.Session, error) {
	if c.sess != nil {
		return c.sess, nil
	}
	if len(c.signers) == 0 {
		return nil, errors.Errorf("no private keys available")
	}
	if c.user == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, errors.Errorf("getting current user: %v", err)
		}
		c.user = currentUser.Username
	}
	config := &ssh.ClientConfig{
		User: c.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				return c.signers, nil
			}),
		},
	}
	client, err := sshDialWithProxy(c.addr, c.proxyCommand, config)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	c.client = client
	c.sess = sess
	c.sess.Stdin = c.stdin
	c.sess.Stdout = c.stdout
	c.sess.Stderr = c.stderr
	return sess, nil
}

func (c *goCryptoCommand) Start() error {
	sess, err := c.ensureSession()
	if err != nil {
		return err
	}
	if c.command == "" {
		return sess.Shell()
	}
	return sess.Start(c.command)
}

func (c *goCryptoCommand) Close() error {
	if c.sess == nil {
		return nil
	}
	err0 := c.sess.Close()
	err1 := c.client.Close()
	if err0 == nil {
		err0 = err1
	}
	c.sess = nil
	c.client = nil
	return err0
}

func (c *goCryptoCommand) Wait() error {
	if c.sess == nil {
		return errors.Errorf("command has not been started")
	}
	err := c.sess.Wait()
	c.Close()
	return err
}

func (c *goCryptoCommand) Kill() error {
	if c.sess == nil {
		return errors.Errorf("command has not been started")
	}
	return c.sess.Signal(ssh.SIGKILL)
}

func (c *goCryptoCommand) SetStdio(stdin io.Reader, stdout, stderr io.Writer) {
	c.stdin = stdin
	c.stdout = stdout
	c.stderr = stderr
}

func (c *goCryptoCommand) StdinPipe() (io.WriteCloser, io.Reader, error) {
	sess, err := c.ensureSession()
	if err != nil {
		return nil, nil, err
	}
	wc, err := sess.StdinPipe()
	return wc, sess.Stdin, err
}

func (c *goCryptoCommand) StdoutPipe() (io.ReadCloser, io.Writer, error) {
	sess, err := c.ensureSession()
	if err != nil {
		return nil, nil, err
	}
	wc, err := sess.StdoutPipe()
	return ioutil.NopCloser(wc), sess.Stdout, err
}

func (c *goCryptoCommand) StderrPipe() (io.ReadCloser, io.Writer, error) {
	sess, err := c.ensureSession()
	if err != nil {
		return nil, nil, err
	}
	wc, err := sess.StderrPipe()
	return ioutil.NopCloser(wc), sess.Stderr, err
}

func splitUserHost(s string) (user, host string) {
	userHost := strings.SplitN(s, "@", 2)
	if len(userHost) == 2 {
		return userHost[0], userHost[1]
	}
	return "", userHost[0]
}
