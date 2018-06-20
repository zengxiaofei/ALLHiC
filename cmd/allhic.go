/**
 * Filename: /Users/htang/code/allhic/main.go
 * Path: /Users/htang/code/allhic
 * Created Date: Wednesday, January 3rd 2018, 11:21:45 am
 * Author: htang
 *
 * Copyright (c) 2018 Haibao Tang
 */

package main

import (
	"os"
	"strconv"
	"time"

	".."
	logging "github.com/op/go-logging"
	"github.com/urfave/cli"
)

// init customizes how cli layout the command interface
// Logo banner (Varsity style):
// http://patorjk.com/software/taag/#p=testall&f=3D-ASCII&t=ALLHIC
func init() {
	cli.AppHelpTemplate = `
     _       _____     _____     ____  ____  _____   ______
    / \     |_   _|   |_   _|   |_   ||   _||_   _|.' ___  |
   / _ \      | |       | |       | |__| |    | | / .'   \_|
  / ___ \     | |   _   | |   _   |  __  |    | | | |
_/ /   \ \_  _| |__/ | _| |__/ | _| |  | |_  _| |_\ ` + "`" + `.___.'\
|____| |____||________||________||____||____||_____|` + "`" + `.____ .'

` + cli.AppHelpTemplate
}

// main is the entrypoint for the entire program, routes to commands
func main() {
	logging.SetBackend(allhic.BackendFormatter)

	app := cli.NewApp()
	app.Compiled = time.Now()
	app.Copyright = "(c) Haibao Tang 2017-2018"
	app.Name = "ALLHIC"
	app.Usage = "Genome scaffolding based on Hi-C data"
	app.Version = allhic.Version

	app.Commands = []cli.Command{
		{
			Name:  "extract",
			Usage: "Extract Hi-C link size distribution",
			UsageText: `
	allhic extract bamfile [options]

Extract function:
Given a bamfile, the goal of the extract step is to calculate an empirical
distribution of Hi-C link size based on intra-contig links. The Extract function
also prepares for the latter steps of ALLHIC.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify distfile, clmfile and bamfile", 1)
				}

				bamfile := c.Args().Get(0)
				p := allhic.Extracter{Bamfile: bamfile}
				p.Run()
				return nil
			},
		},
		{
			Name:  "prune",
			Usage: "Prune bamfile to remove weak links",
			UsageText: `
	allhic prune bamfile [options]

Prune function:
Given a bamfile, the goal of the pruning step is to remove all inter-allelic
links, then it is possible to reconstruct allele-separated assemblies.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify bamfile", 1)
				}

				bamfile := c.Args().Get(0)
				p := allhic.Pruner{Bamfile: bamfile}
				p.Run()
				return nil
			},
		},
		{
			Name:  "anchor",
			Usage: "Anchor contigs based on an iterative merging method",
			UsageText: `
	allhic anchor bamfile [options]

Anchor function:
Given a bamfile, we anchor contigs based on an iterative merging method similar
to the method used in 3D-DNA. The method is based on maximum weight matching
of the contig linkage graph.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify bamfile", 1)
				}

				bamfile := c.Args().Get(0)
				p := allhic.Anchorer{Bamfile: bamfile}
				p.Run()
				return nil
			},
		},
		{
			Name:  "partition",
			Usage: "Separate bamfile into k groups",
			UsageText: `
	allhic partition enrichment.txt distance.txt k [options]

Partition function:
Given a target k, number of partitions, the goal of the partitioning is to
separate all the contigs into separate clusters. As with all clustering
algorithm, there is an optimization goal here. The LACHESIS algorithm is
a hierarchical clustering algorithm using average links. The distfile can be
generated with the "extract" sub-command.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 3 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify distfile", 1)
				}

				contigsfile := c.Args().Get(0)
				distfile := c.Args().Get(1)
				k, _ := strconv.Atoi(c.Args().Get(2))
				p := allhic.Partitioner{Contigsfile: contigsfile, Distfile: distfile, K: k}
				p.Run()
				return nil
			},
		},
		{
			Name:  "optimize",
			Usage: "Order-and-orient tigs in a group",
			UsageText: `
	allhic optimize clmfile [options]

Optimize function:
Given a set of Hi-C contacts between contigs, as specified in the
clmfile, reconstruct the highest scoring ordering and orientations
for these contigs.
`,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "skipGA",
					Usage: "Skip GA step",
				},
				cli.Float64Flag{
					Name:  "mutpb",
					Usage: "Mutation prob in GA",
					Value: .2,
				},
				cli.Float64Flag{
					Name:  "cxpb",
					Usage: "Crossover prob in GA",
					Value: .7,
				},
			},
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify clmfile", 1)
				}

				clmfile := c.Args().Get(0)
				runGA := !c.Bool("skipGA")
				mutpb := c.Float64("mutpb")
				cxpb := c.Float64("cxpb")
				p := allhic.Optimizer{Clmfile: clmfile, RunGA: runGA, MutProb: mutpb, CrossProb: cxpb}
				p.Run()
				return nil
			},
		},
		{
			Name:  "build",
			Usage: "Build genome release",
			UsageText: `
	allhic build tourfile contigs.fasta [options]

Build function:
Convert the tourfile into the standard AGP file, which is then converted
into a FASTA genome release.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 2 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify tourfile and fastafile", 1)
				}

				tourfile := c.Args().Get(0)
				fastafile := c.Args().Get(1)
				p := allhic.Builder{Tourfile: tourfile, Fastafile: fastafile}
				p.Run()
				return nil
			},
		},
		{
			Name:  "assess",
			Usage: "Assess the orientations of contigs",
			UsageText: `
	allhic assess bamfile

Assess function:
Compute the posterior probability of contig orientations after scaffolding
as a quality assessment step.
`,
			Action: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					cli.ShowSubcommandHelp(c)
					return cli.NewExitError("Must specify bamfile", 1)
				}

				bamfile := c.Args().Get(0)
				p := allhic.Assesser{Bamfile: bamfile}
				p.Run()
				return nil
			},
		},
	}

	app.Run(os.Args)
}
