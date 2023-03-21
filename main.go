package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/kardianos/service"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

var currentDirectory = getCurrentDirectory()

type program struct{}

type PowerShell struct {
	powerShell string
}

func NewPowerShell() *PowerShell {
	ps, _ := exec.LookPath("powershell.exe")
	return &PowerShell{
		powerShell: ps,
	}
}

func (p *PowerShell) ExecuteScript(script string) (error, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(p.powerShell, "-NonInteractive", "-ExecutionPolicy", "RemoteSigned", "-File", script)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return err, stdout.String(), stderr.String()
}

func (p *program) Start(s service.Service) error {
	_ = s

	go p.run()
	return nil
}

func (p *program) run() {
	log.Printf("Starting EC2 prep...")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	client := imds.NewFromConfig(cfg)
	instanceIdentityDocument, err := client.GetInstanceIdentityDocument(context.TODO(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		log.Fatal(err)
	}

	instanceId := instanceIdentityDocument.InstanceID

	instancePath := fmt.Sprintf("%s\\Instance\\%s", currentDirectory, instanceId)
	log.Printf("Checking if instance has already been processed.")

	if _, err := os.Stat(instancePath); !os.IsNotExist(err) {
		log.Printf("Instance has already been processed, exiting...")
		os.Exit(0)
	}

	err = os.Mkdir(instancePath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	scriptsPath := fmt.Sprintf("%s\\Scripts", currentDirectory)
	log.Printf("Reading files from %s", scriptsPath)

	files, _ := os.ReadDir(scriptsPath)

	ps := NewPowerShell()

	for _, file := range files {
		filePath := fmt.Sprintf("%s\\%s", scriptsPath, file.Name())
		log.Printf("Running script %s", filePath)

		err, stdout, stderr := ps.ExecuteScript(filePath)
		if err != nil {
			log.Fatal(err)
		}

		if stdout != "" {
			log.Print(stdout)
		}

		if stderr != "" {
			log.Printf("Error while running script %s", filePath)
			log.Print(stderr)
		}
	}

	log.Printf("Completed EC2 prep.")
	os.Exit(0)
}

func (p *program) Stop(s service.Service) error {
	_ = s
	return nil
}

func main() {
	installFlag := flag.Bool("install", false, "Install the Windows service.")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall the Windows service.")
	flag.Parse()

	options := make(map[string]interface{})
	options["DelayedAutoStart"] = true

	svcConfig := &service.Config{
		Name:        "EC2Prep",
		DisplayName: "EC2 Prep",
		Description: "Runs powershell scripts on instance first boot",
		Option:      options,
	}
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if *installFlag == true {
		initializeDirectories()
		err := s.Install()
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if *uninstallFlag == true {
		err := s.Uninstall()
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	setLogLocation()

	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func initializeDirectories() {
	instanceDirectory := fmt.Sprintf("%s\\Instance", currentDirectory)
	if _, err := os.Stat(instanceDirectory); os.IsNotExist(err) {
		err = os.Mkdir(instanceDirectory, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}
	scriptsDirectory := fmt.Sprintf("%s\\Scripts", currentDirectory)
	if _, err := os.Stat(scriptsDirectory); os.IsNotExist(err) {
		err = os.Mkdir(scriptsDirectory, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func getCurrentDirectory() string {
	currentPath, _ := os.Executable()
	parentPath := filepath.Dir(currentPath)

	return parentPath
}

func setLogLocation() {
	logPath := fmt.Sprintf("%s\\ec2prep.log", currentDirectory)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)
}
