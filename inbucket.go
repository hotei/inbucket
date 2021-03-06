/*
	This is the inbucket daemon launcher
*/
package main

import (
	"expvar"
	"flag"
	"fmt"
	"github.com/jhillyerd/inbucket/config"
	"github.com/jhillyerd/inbucket/log"
	"github.com/jhillyerd/inbucket/pop3d"
	"github.com/jhillyerd/inbucket/smtpd"
	"github.com/jhillyerd/inbucket/web"
	golog "log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// Build info, populated during linking by goxc
	VERSION    = "1.0"
	BUILD_DATE = "undefined"

	// Command line flags
	help    = flag.Bool("help", false, "Displays this help")
	pidfile = flag.String("pidfile", "none", "Write our PID into the specified file")
	logfile = flag.String("logfile", "stderr", "Write out log into the specified file")

	// startTime is used to calculate uptime of Inbucket
	startTime = time.Now()

	// The file we send log output to, will be nil for stderr or stdout
	logf *os.File

	// Server instances
	smtpServer *smtpd.Server
	pop3Server *pop3d.Server
)

func main() {
	config.VERSION = VERSION
	config.BUILD_DATE = BUILD_DATE

	flag.Parse()
	if *help {
		flag.Usage()
		return
	}

	// Load & Parse config
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	err := config.LoadConfig(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handler
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGTERM)
	go signalProcessor(sigChan)

	// Configure logging, close std* fds
	level, _ := config.Config.String("logging", "level")
	log.SetLogLevel(level)

	if *logfile != "stderr" {
		// stderr is the go logging default
		if *logfile == "stdout" {
			// set to stdout
			golog.SetOutput(os.Stdout)
		} else {
			err = openLogFile()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v", err)
				os.Exit(1)
			}
			defer closeLogFile()

			// close std* streams
			os.Stdout.Close()
			os.Stderr.Close() // Warning: this will hide panic() output
			os.Stdin.Close()
			os.Stdout = logf
			os.Stderr = logf
		}
	}

	log.LogInfo("Inbucket %v (%v) starting...", config.VERSION, config.BUILD_DATE)

	// Write pidfile if requested
	// TODO: Probably supposed to remove pidfile during shutdown
	if *pidfile != "none" {
		pidf, err := os.Create(*pidfile)
		if err != nil {
			log.LogError("Failed to create %v: %v", *pidfile, err)
			os.Exit(1)
		}
		defer pidf.Close()
		fmt.Fprintf(pidf, "%v\n", os.Getpid())
	}

	// Grab our datastore
	ds := smtpd.DefaultFileDataStore()

	// Start HTTP server
	web.Initialize(config.GetWebConfig(), ds)
	go web.Start()

	// Start POP3 server
	pop3Server = pop3d.New()
	go pop3Server.Start()

	// Startup SMTP server, block until it exits
	smtpServer = smtpd.NewSmtpServer(config.GetSmtpConfig(), ds)
	smtpServer.Start()

	// Wait for active connections to finish
	smtpServer.Drain()
	pop3Server.Drain()
}

// openLogFile creates or appends to the logfile passed on commandline
func openLogFile() error {
	// use specified log file
	var err error
	logf, err = os.OpenFile(*logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Failed to create %v: %v\n", *logfile, err)
	}
	golog.SetOutput(logf)
	log.LogTrace("Opened new logfile")
	return nil
}

// closeLogFile closes the current logfile
func closeLogFile() error {
	log.LogTrace("Closing logfile")
	return logf.Close()
}

// signalProcessor is a goroutine that handles OS signals
func signalProcessor(c <-chan os.Signal) {
	for {
		sig := <-c
		switch sig {
		case syscall.SIGHUP:
			// Rotate logs if configured
			if logf != nil {
				log.LogInfo("Recieved SIGHUP, cycling logfile")
				closeLogFile()
				openLogFile()
			} else {
				log.LogInfo("Ignoring SIGHUP, logfile not configured")
			}
		case syscall.SIGTERM:
			// Initiate shutdown
			log.LogInfo("Received SIGTERM, shutting down")
			go timedExit()
			web.Stop()
			if smtpServer != nil {
				smtpServer.Stop()
			} else {
				log.LogError("smtpServer was nil during shutdown")
			}
		}
	}
}

// timedExit is called as a goroutine during shutdown, it will force an exit after 15 seconds
func timedExit() {
	time.Sleep(15 * time.Second)
	log.LogError("Inbucket clean shutdown timed out, forcing exit")
	os.Exit(0)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage of inbucket [options] <conf file>:")
		flag.PrintDefaults()
	}

	expvar.Publish("uptime", expvar.Func(uptime))
}

// uptime() is published as an expvar
func uptime() interface{} {
	return time.Since(startTime) / time.Second
}
