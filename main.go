package main

import (
	"bytes"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azuredevops-deployment-operator/config"
	"github.com/webdevops/azuredevops-deployment-operator/operator"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"text/template"
)

const (
	Author = "webdevops.io"
)

var (
	argparser *flags.Parser

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

var opts = config.Opts{}

func main() {
	initArgparser()

	log.Infof("starting AzureDevops Deployment operator v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)

	op := operator.AzureDevopsOperator{
		Opts:         opts.AzureDevops,
		Notification: opts.Notification,
		Config:       parseAppConfig(opts.ConfigPath),
	}
	op.Init()
	op.Start()

	log.Infof("starting http server on %s", opts.ServerBind)
	startHttpServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func parseAppConfig(path string) (conf config.Config) {
	var configRaw []byte

	conf = config.Config{}

	log.Infof("reading configuration from file %v", path)
	if data, err := ioutil.ReadFile(path); err == nil {
		configRaw = data
	} else {
		panic(err)
	}

	log.Info("preprocessing with template engine")
	var tmplBytes bytes.Buffer
	parsedConfig, err := template.New("yaml").Funcs(sprig.TxtFuncMap()).Parse(string(configRaw))
	if err != nil {
		panic(err)
	}

	if err := parsedConfig.Execute(&tmplBytes, nil); err != nil {
		panic(err)
	}

	log.Info("parsing configuration")
	if err := yaml.Unmarshal(tmplBytes.Bytes(), &conf); err != nil {
		panic(err)
	}

	return
}

// start and handle prometheus handler
func startHttpServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
