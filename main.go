package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/mattn/go-shellwords"
	"github.com/tj/go-spin"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	verbose                = flag.Bool("v", false, "Show verbose logging")
	quiet                  = flag.Bool("q", true, "Show quiet logging")
	force                  = flag.Bool("f", false, "Force firmware update")
	copier                 = flag.Bool("c", false, "Copy bin_file to bin_save")
	core                   = flag.String("core", "", "Core version")
	from                   = flag.String("from", "", "Original file location")
	to                     = flag.String("to", "", "Save file location")
	dfu_path               = flag.String("dfu", "", "Location of dfu-util binaries")
	bin_file_name          = flag.String("bin", "", "Location of sketch binary")
	com_port               = flag.String("port", "", "Upload serial port")
)

const Version = "2.0.0"

const dfu_flags = "-d,0483:df11"

func PrintlnVerbose(a ...interface{}) {
	if *verbose {
		fmt.Println(a...)
	}
}

func PrintVerbose(a ...interface{}) {
	if *verbose {
		fmt.Print(a...)
	}
}

func main_load() {

	if *dfu_path == "" {
		fmt.Println("Need to specify dfu-util location")
		os.Exit(1)
	}

	if *bin_file_name == "" && *force == false {
		fmt.Println("Need to specify a binary location or force FW update")
		os.Exit(1)
	}

	// Remove ""s from the strings
	*dfu_path = strings.Replace(*dfu_path, "\"", "", -1)
	*bin_file_name = strings.Replace(*bin_file_name, "\"", "", -1)

	dfu := *dfu_path + "/dfu-util"
	dfu = filepath.ToSlash(dfu)

	PrintlnVerbose("Serial Port: " + *com_port)
	PrintlnVerbose("BIN FILE " + *bin_file_name)

	counter := 0
	board_found := false

	dfu_search_command := []string{dfu, dfu_flags, "-l"}
	var err error

	for counter < 100 && board_found == false {
		if counter%10 == 0 {
			PrintlnVerbose("Waiting for device...")
		}
		err, found, _ := launchCommandAndWaitForOutput(dfu_search_command, "Internal", false, false)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if counter == 40 {
			fmt.Println("Flashing is taking longer than expected")
			// fmt.Println("Try pressing MASTER_RESET button")
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
		fmt.Println("ERROR: Timed out waiting for Arduino Star OTTO on " + *com_port)
		os.Exit(1)
	}

	// Finally flash the sketch

	if *bin_file_name == "" {
		os.Exit(0)
	}

	// 9600 trick linux

	if runtime.GOOS == "linux" {    // also can be specified to FreeBSD
		fmt.Println("Unix/Linux type OS detected")
		trick_9600 := []string{"stty", "-F", *com_port, "9600"}
		err, _, _ = launchCommandAndWaitForOutput(trick_9600, "", true, false)
	}

	// time.Sleep(3 * time.Second)

	// dfu-util reference string
	// dfu_download := []string{dfu, dfu_flags, "-D", *bin_file_name, "-v", "--alt", "7", "-R"}

	// dfu-util v0.9
	// dfu_download := []string{dfu, dfu_flags, "-a", "0", "-O", *bin_file_name, "-s", "0x08000000", "-f", "0x08000000"}
	// err, _, _ = launchCommandAndWaitForOutput(dfu_download, "", true, false)

	// dfu-util v0.8
	dfu_download := []string{dfu, dfu_flags, "-a", "0", "-D", *bin_file_name, "-s", "0x08000000"}
	dfu_resetstm32 := []string{dfu, dfu_flags, "-a", "0", "--reset-stm32"}
	err, _, _ = launchCommandAndWaitForOutput(dfu_download, "", true, false)
	err, _, _ = launchCommandAndWaitForOutput(dfu_resetstm32, "", true, false)

	if err == nil {
		fmt.Println("SUCCESS: Sketch will execute in about 5 seconds.")
		os.Exit(0)
	} else {
		fmt.Println("ERROR: Upload failed on " + *com_port)
		os.Exit(1)
	}
}

func main_debug(args []string) {

	if len(args) < 1 {
		fmt.Println("Not enough arguments")
		os.Exit(1)
	}

	*verbose = true

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
			err, _, _ = launchCommandAndWaitForOutput(cmd, "", true, false)
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
	name := filepath.Base(os.Args[0])

	flag.Parse()

	PrintlnVerbose(name + " " + Version + " - compiled with " + runtime.Version())

	if *copier {
		if *from == "" || *to == "" {
			fmt.Println("ERROR: need -from and -to arguments")
			os.Exit(1)
		}
		*from = strings.Replace(*from, "\"", "", -1)
		*to = strings.Replace(*to, "\"", "", -1)
		copy(*from, *to)
		os.Exit(0)
	}

	if strings.Contains(name, "load") {
		fmt.Println("Starting download script...")
		main_load()
	}

	if strings.Contains(name, "debug") {
		fmt.Println("Starting debug script...")
		main_debug(os.Args[1:])
	}

	fmt.Println("Wrong executable name")
	os.Exit(1)
}

// Copy a file
func copy(source, destination string) {
	// Open original file
	originalFile, err := os.Open(source)
	if err != nil {
		os.Exit(1)
	}
	defer originalFile.Close()

	// Create new file
	newFile, err := os.Create(destination)
	if err != nil {
		os.Exit(1)
	}
	defer newFile.Close()

	// Copy the bytes to destination from source
	_, err = io.Copy(newFile, originalFile)
	if err != nil {
		os.Exit(1)
	}

	// Commit the file contents
	// Flushes memory to disk
	err = newFile.Sync()
	if err != nil {
		os.Exit(1)
	}
}

func searchVersionInDFU(file string, string_to_search string) bool {
	read, _ := ioutil.ReadFile(file)
	return strings.Contains(string(read), string_to_search)
}

func launchCommandAndWaitForOutput(command []string, stringToSearch string, print_output bool, show_spinner bool) (error, bool, string) {
	oscmd := exec.Command(command[0], command[1:]...)
	tellCommandNotToSpawnShell(oscmd)
	stdout, _ := oscmd.StdoutPipe()
	stderr, _ := oscmd.StderrPipe()
	multi := io.MultiReader(stdout, stderr)

	s := spin.New()
	s.Set(spin.Spin1)

	if print_output && *verbose {
		oscmd.Stdout = os.Stdout
		oscmd.Stderr = os.Stderr
	}
	err := oscmd.Start()
	in := bufio.NewScanner(multi)
	in.Split(bufio.ScanRunes)
	found := false
	out := ""
	for in.Scan() {

		if show_spinner {
			fmt.Printf("\r %s", s.Next())
		}

		out += in.Text()
		if stringToSearch != "" {
			if strings.Contains(out, stringToSearch) {
				found = true
			}
		}
	}
	err = oscmd.Wait()
	if show_spinner {
		fmt.Println("")
	}
	return err, found, out
}

func launchCommandBackground(command []string, stringToSearch string, print_output bool) (error, bool) {
	oscmd := exec.Command(command[0], command[1:]...)
	tellCommandNotToSpawnShell(oscmd)
	err := oscmd.Start()
	return err, false
}
