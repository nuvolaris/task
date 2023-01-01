package taskmain

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/go-task/task/v3"
	"github.com/spf13/pflag"

	"github.com/go-task/task/v3/args"
	"github.com/go-task/task/v3/taskfile"
)

var (
	version = ""
)

const usage = `Usage: task [-ilfwvsd] [--init] [--list] [--force] [--watch] [--verbose] [--silent] [--dir] [--taskfile] [--dry] [--summary] [task...]

Runs the specified task(s). Falls back to the "default" task if no task name
was specified, or lists all tasks if an unknown task name was specified.

Example: 'task hello' with the following 'Taskfile.yml' file will generate an
'output.txt' file with the content "hello".

'''
version: '3'
tasks:
  hello:
    cmds:
      - echo "I am going to write a file named 'output.txt' now."
      - echo "hello" > output.txt
    generates:
      - output.txt
'''

Options:
`

func Task(arguments []string) {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	pflags := pflag.FlagSet{}
	pflags.Usage = func() {
		log.Print(usage)
		pflag.PrintDefaults()
	}

	var (
		versionFlag bool
		helpFlag    bool
		init        bool
		list        bool
		listAll     bool
		status      bool
		force       bool
		watch       bool
		verbose     bool
		silent      bool
		dry         bool
		summary     bool
		exitCode    bool
		parallel    bool
		concurrency int
		dir         string
		entrypoint  string
		output      taskfile.Output
		color       bool
		interval    string
	)

	pflags.BoolVar(&versionFlag, "version", false, "show Task version")
	pflags.BoolVarP(&helpFlag, "help", "h", false, "shows Task usage")
	pflags.BoolVarP(&init, "init", "i", false, "creates a new Taskfile.yaml in the current folder")
	pflags.BoolVarP(&list, "list", "l", false, "lists tasks with description of current Taskfile")
	pflags.BoolVarP(&listAll, "list-all", "a", false, "lists tasks with or without a description")
	pflags.BoolVar(&status, "status", false, "exits with non-zero exit code if any of the given tasks is not up-to-date")
	pflags.BoolVarP(&force, "force", "f", false, "forces execution even when the task is up-to-date")
	pflags.BoolVarP(&watch, "watch", "w", false, "enables watch of the given task")
	pflags.BoolVarP(&verbose, "verbose", "v", false, "enables verbose mode")
	pflags.BoolVarP(&silent, "silent", "s", false, "disables echoing")
	pflags.BoolVarP(&parallel, "parallel", "p", false, "executes tasks provided on command line in parallel")
	pflags.BoolVarP(&dry, "dry", "n", false, "compiles and prints tasks in the order that they would be run, without executing them")
	pflags.BoolVar(&summary, "summary", false, "show summary about a task")
	pflags.BoolVarP(&exitCode, "exit-code", "x", false, "pass-through the exit code of the task command")
	pflags.StringVarP(&dir, "dir", "d", "", "sets directory of execution")
	pflags.StringVarP(&entrypoint, "taskfile", "t", "", `choose which Taskfile to run. Defaults to "Taskfile.yml"`)
	pflags.StringVarP(&output.Name, "output", "o", "", "sets output style: [interleaved|group|prefixed]")
	pflags.StringVar(&output.Group.Begin, "output-group-begin", "", "message template to print before a task's grouped output")
	pflags.StringVar(&output.Group.End, "output-group-end", "", "message template to print after a task's grouped output")
	pflags.BoolVarP(&color, "color", "c", true, "colored output. Enabled by default. Set flag to false or use NO_COLOR=1 to disable")
	pflags.IntVarP(&concurrency, "concurrency", "C", 0, "limit number tasks to run concurrently")
	pflags.StringVarP(&interval, "interval", "I", "5s", "interval to watch for changes")
	pflags.Parse(arguments)

	if versionFlag {
		fmt.Printf("Task version: %s\n", getVersion())
		return
	}

	if helpFlag {
		pflags.Usage()
		return
	}

	if init {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		if err := task.InitTaskfile(os.Stdout, wd); err != nil {
			log.Fatal(err)
		}
		return
	}

	if dir != "" && entrypoint != "" {
		log.Fatal("task: You can't set both --dir and --taskfile")
		return
	}
	if entrypoint != "" {
		dir = filepath.Dir(entrypoint)
		entrypoint = filepath.Base(entrypoint)
	}

	if output.Name != "group" {
		if output.Group.Begin != "" {
			log.Fatal("task: You can't set --output-group-begin without --output=group")
			return
		}
		if output.Group.End != "" {
			log.Fatal("task: You can't set --output-group-end without --output=group")
			return
		}
	}

	e := task.Executor{
		Force:       force,
		Watch:       watch,
		Verbose:     verbose,
		Silent:      silent,
		Dir:         dir,
		Dry:         dry,
		Entrypoint:  entrypoint,
		Summary:     summary,
		Parallel:    parallel,
		Color:       color,
		Concurrency: concurrency,
		Interval:    interval,

		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,

		OutputStyle: output,
	}

	if (list || listAll) && silent {
		e.ListTaskNames(listAll)
		return
	}

	if err := e.Setup(); err != nil {
		log.Fatal(err)
	}
	v, err := e.Taskfile.ParsedVersion()
	if err != nil {
		log.Fatal(err)
		return
	}

	if list {
		if ok := e.ListTasks(task.FilterOutInternal(), task.FilterOutNoDesc()); !ok {
			//e.Logger.Outf(Yellow, "task: No tasks with description available. Try --list-all to list all tasks")
			fmt.Printf("task: No tasks with description available. Try --list-all to list all tasks")
		}
		return
	}

	if listAll {
		if ok := e.ListTasks(task.FilterOutInternal()); !ok {
			//e.Logger.Outf(Yellow, "task: No tasks available")
			fmt.Printf("task: No tasks available")
		}
		return
	}

	var (
		calls   []taskfile.Call
		globals *taskfile.Vars
	)

	tasksAndVars, cliArgs, err := getArgs()
	if err != nil {
		log.Fatal(err)
	}

	if v >= 3.0 {
		calls, globals = args.ParseV3(tasksAndVars...)
	} else {
		calls, globals = args.ParseV2(tasksAndVars...)
	}

	globals.Set("CLI_ARGS", taskfile.Var{Static: cliArgs})
	e.Taskfile.Vars.Merge(globals)

	if !watch {
		e.InterceptInterruptSignals()
	}

	ctx := context.Background()

	if status {
		if err := e.Status(ctx, calls...); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := e.Run(ctx, calls...); err != nil {
		//e.Logger.Errf(Red, "%v", err)
		fmt.Errorf("%v", err)

		if exitCode {
			if err, ok := err.(*task.TaskRunError); ok {
				os.Exit(err.ExitCode())
			}
		}
		os.Exit(1)
	}
}

func getArgs() ([]string, string, error) {
	var (
		args          = pflag.Args()
		doubleDashPos = pflag.CommandLine.ArgsLenAtDash()
	)

	if doubleDashPos == -1 {
		return args, "", nil
	}

	var quotedCliArgs []string
	for _, arg := range args[doubleDashPos:] {
		quotedCliArg, err := syntax.Quote(arg, syntax.LangBash)
		if err != nil {
			return nil, "", err
		}
		quotedCliArgs = append(quotedCliArgs, quotedCliArg)
	}
	return args[:doubleDashPos], strings.Join(quotedCliArgs, " "), nil
}

func getVersion() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}

	version = info.Main.Version
	if info.Main.Sum != "" {
		version += fmt.Sprintf(" (%s)", info.Main.Sum)
	}

	return version
}
