package main

import (
	"bufio"
	"fmt"
	"github.com/mattn/go-shellwords"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Version = "1.0.0"

var verbose bool

func PrintlnVerbose(a ...interface{}) {
	if verbose {
		fmt.Println(a...)
	}
}

func main_load(args []string) {

	// ARG 1: Path to binaries
	// ARG 2: BIN File to download
	// ARG 3: TTY port to use.
	// ARG 4: quiet/verbose
	// path may contain \ need to change all to /

	if len(args) < 4 {
		fmt.Println("Not enough arguments")
		os.Exit(1)
	}

	bin_path := args[0]
	dfu := bin_path + "/dfu-util"
	dfu = filepath.ToSlash(dfu)
	dfu_flags := "-d,0483:DF11"

	bin_file_name := args[1]

	com_port := args[2]
	verbosity := args[3]

	if verbosity == "quiet" {
		verbose = false
	} else {
		verbose = true
	}

	PrintlnVerbose("Args to shell:", args)
	PrintlnVerbose("Serial Port: " + com_port)
	PrintlnVerbose("BIN FILE " + bin_file_name)

	counter := 0
	board_found := false

	if runtime.GOOS == "darwin" {
		library_path := os.Getenv("DYLD_LIBRARY_PATH")
		if !strings.Contains(library_path, bin_path) {
			os.Setenv("DYLD_LIBRARY_PATH", bin_path+":"+library_path)
		}
	}

	dfu_search_command := []string{dfu, dfu_flags, "-l"}

	for counter < 100 && board_found == false {
		if counter%10 == 0 {
			PrintlnVerbose("Waiting for device...")
		}
		err, found, _ := launchCommandAndWaitForOutput(dfu_search_command, "Internal", false)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if counter == 40 {
			fmt.Println("Flashing is taking longer than expected")
		}
		if found == true {
			board_found = true
			PrintlnVerbose("Device found!")
			break
		}
		time.Sleep(100 * time.Millisecond)
		counter++
	}

	if board_found == false {
		fmt.Println("ERROR: Timed out waiting for Arduino Star OTTO on " + com_port)
		os.Exit(1)
	}

	// dfu_download := []string{dfu, dfu_flags, "-D", bin_file_name, "-v", "--alt", "7", "-R"}
	dfu_download := []string{dfu, dfu_flags, "-a", "0", "-O", bin_file_name, "-s", "0x08000000", "-f", "0x08000000"}

	// cmd := exec.Command(dfu, dfu_flags)
	// printCommand(cmd)
	// output, err := cmd.CombinedOutput()
	// printError(err)
	// printOutput(output) // => go version go1.3 darwin/amd64

	err, _, _ := launchCommandAndWaitForOutput(dfu_download, "", true)

	if err == nil {
		fmt.Println("SUCCESS: Sketch will execute in about 5 seconds.")
		os.Exit(0)
	} else {
		fmt.Println("ERROR: Upload failed on " + com_port)
		os.Exit(1)
	}
}

func main_debug(args []string) {

	if len(args) < 1 {
		fmt.Println("Not enough arguments")
		os.Exit(1)
	}

	verbose = true

	type Command struct {
		command    string
		background bool
	}

	var commands []Command

	fullcmdline := strings.Join(args[:], "")
	temp_commands := strings.Split(fullcmdline, ";")
	for _, command := range temp_commands {
		background_commands := strings.Split(command, "&")
		for i, command := range background_commands {
			var cmd Command
			cmd.background = (i < len(background_commands)-1)
			cmd.command = command
			commands = append(commands, cmd)
		}
	}

	var err error

	for _, command := range commands {
		fmt.Println("command: " + command.command)
		cmd, _ := shellwords.Parse(command.command)
		fmt.Println(cmd)
		if command.background == false {
			err, _, _ = launchCommandAndWaitForOutput(cmd, "", true)
		} else {
			err, _ = launchCommandBackground(cmd, "", true)
		}
		if err != nil {
			fmt.Println("ERROR: Command \" " + command.command + " \" failed")
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func main() {
	name := os.Args[0]
	args := os.Args[1:]

	if strings.Contains(name, "load") {
		fmt.Println("Starting download script...")
		main_load(args)
	}

	if strings.Contains(name, "debug") {
		fmt.Println("Starting debug script...")
		main_debug(args)
	}

	fmt.Println("Wrong executable name")
	os.Exit(1)
}

func launchCommandAndWaitForOutput(command []string, stringToSearch string, print_output bool) (error, bool, string) {
	// time.Sleep(2 * time.Second)
	oscmd := exec.Command(command[0], command[1:]...)
	tellCommandNotToSpawnShell(oscmd)
	stdout, _ := oscmd.StdoutPipe()
	stderr, _ := oscmd.StderrPipe()
	multi := io.MultiReader(stderr, stdout)
	err := oscmd.Start()
	in := bufio.NewScanner(multi)
	in.Split(bufio.ScanLines)
	found := false
	out := ""
	for in.Scan() {
		if print_output {
			PrintlnVerbose(in.Text())
		}
		out += in.Text() + "\n"
		if stringToSearch != "" {
			if strings.Contains(in.Text(), stringToSearch) {
				found = true
			}
		}
	}
	err = oscmd.Wait()
	return err, found, out
}

func launchCommandBackground(command []string, stringToSearch string, print_output bool) (error, bool) {
	oscmd := exec.Command(command[0], command[1:]...)
	tellCommandNotToSpawnShell(oscmd)
	err := oscmd.Start()
	return err, false
}

func printCommand(cmd *exec.Cmd) {
  fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func printError(err error) {
  if err != nil {
    os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
  }
}

func printOutput(outs []byte) {
  if len(outs) > 0 {
    fmt.Printf("==> Output: %s\n", string(outs))
  }
}
