package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gobuffalo/packr/v2"
	"github.com/gregdhill/go-openrpc/generate"
	"github.com/gregdhill/go-openrpc/parse"
	"github.com/gregdhill/go-openrpc/types"
)

var (
	pkgDir         string
	specFile       string
	cliGen         bool
	cliCommandName string
)

func init() {
	flag.StringVar(&pkgDir, "dir", "rpc", "set the target directory")
	flag.StringVar(&specFile, "spec", "", "the openrpc compliant spec")
	flag.StringVar(&cliCommandName, "cli.name", "CHANGEME", "With -cli, names binary program. Default is FIXME.")
	flag.BoolVar(&cliGen, "cli", false, "Toggle CLI program generation")
}

func readSpec(file string) (*types.OpenRPCSpec1, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	spec := types.NewOpenRPCSpec1()
	err = json.Unmarshal(data, spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

func run() error {
	flag.Parse()
	if specFile == "" {
		return fmt.Errorf("spec file is required")
	}

	openrpc, err := readSpec(specFile)
	if err != nil {
		return err
	}

	parse.GetTypes(openrpc, openrpc.Objects)
	box := packr.New("template", "./templates")

	if err = generate.WriteFile(box, "server", pkgDir, openrpc); err != nil {
		return err
	}

	if err = generate.WriteFile(box, "types", pkgDir, openrpc); err != nil {
		return err
	}

	if err = generate.WriteDocMd(box, "doc", pkgDir, openrpc); err != nil {
		return fmt.Errorf("MIERROR %s", err)
	}

	if cliGen {
		generate.ProgramName = cliCommandName
		if err = generate.WriteFile(box, "cli", "main", openrpc); err != nil {
			return err
		}

		if err = generate.WriteFile(box, "cli_cmd", "cmd", openrpc); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
