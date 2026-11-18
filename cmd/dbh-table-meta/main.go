// dbh-table-meta generates, from the model structs of a Go package, their
// Meta() methods, the model list and the SQL schema scripts.
//
// It is a thin wrapper: the work is done by github.com/pablo-botella/dbh/tool.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pablo-botella/dbh/tool"
)

func main() {
	var o tool.Options
	flag.StringVar(&o.Dir, "dir", ".", "folder with the Go sources of the models")
	flag.StringVar(&o.MetaFile, "meta", "", "write the Meta() methods to this Go `file` (relative to -dir)")
	flag.StringVar(&o.ListFile, "list", "", "write the model list to this Go `file` (relative to -dir)")
	flag.StringVar(&o.ListVar, "list-var", tool.DefaultListVar, "`name` of the model list variable")
	flag.StringVar(&o.SQLDir, "sql", "", "write one SQL script per dialect into this `folder`")
	flag.StringVar(&o.Dialects, "dialects", tool.DefaultDialects, "comma-separated SQL `dialects`")
	flag.StringVar(&o.Group, "group", "", "only the models of this group `name`; empty means all")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected argument: %s\n\n", flag.Arg(0))
		usage()
		os.Exit(2)
	}

	res, err := tool.Run(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbh-table-meta:", err)
		os.Exit(1)
	}
	for _, f := range res.Files {
		fmt.Println("Generated:", f)
	}
	fmt.Printf("Processed %d models\n", res.Models)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dbh-table-meta [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Each output is written only when its option is given;")
	fmt.Fprintln(os.Stderr, "at least one of -meta, -list or -sql is required.")
	fmt.Fprintln(os.Stderr)
	flag.PrintDefaults()
}
