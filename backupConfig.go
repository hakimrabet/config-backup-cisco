package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func readBuffForString(whattoexpect string, sshOut io.Reader, buffRead chan<- string) {
	buf := make([]byte, 1000)
	n, err := sshOut.Read(buf) //this reads the ssh terminal
	waitingString := ""
	if err == nil {
		waitingString = string(buf[:n])
	}
	for (err == nil) && (!strings.Contains(waitingString, whattoexpect)) {
		n, err = sshOut.Read(buf)
		waitingString += string(buf[:n])
		//fmt.Println(waitingString)
	}
	buffRead <- waitingString
}
func readBuff(whattoexpect string, sshOut io.Reader, timeoutSeconds int) string {
	ch := make(chan string)
	go func(whattoexpect string, sshOut io.Reader) {
		buffRead := make(chan string)
		go readBuffForString(whattoexpect, sshOut, buffRead)
		select {
		case ret := <-buffRead:
			ch <- ret
		case <-time.After(time.Duration(timeoutSeconds) * time.Second):
			handleError(fmt.Errorf("%d", timeoutSeconds), true, "Waiting for \""+whattoexpect+"\" took longer than %s seconds, perhaps you've entered incorrect details?")
		}
	}(whattoexpect, sshOut)
	return <-ch
}
func writeBuff(command string, sshIn io.WriteCloser) (int, error) {
	returnCode, err := sshIn.Write([]byte(command + "\r"))
	return returnCode, err
}
func handleError(e error, fatal bool, customMessage ...string) {
	var errorMessage string
	if e != nil {
		if len(customMessage) > 0 {
			errorMessage = strings.Join(customMessage, " ")
		} else {
			errorMessage = "%s"
		}
		if fatal == true {
			log.Fatalf(errorMessage, e)
		} else {
			log.Print(errorMessage, e)
		}
	}
}
func main() {
	//argsWithoutProg := os.Args[1:]
	/*if len(os.Args) == 2 {
		if os.Args[1] == "-h" {
			fmt.Println("Version 0.1")
			fmt.Println("This program build by hakimrabet for backup network devices with tftp")
			fmt.Println("help\n", os.Args[0], "ip_address", "username","password", "enable_password", "tftp-server")
		}
		return
	}
	if len(os.Args) != 6 {
		fmt.Println("for help run with -h")
		fmt.Println("Usage:", os.Args[0], "ip_address", "username","password","enable_password", "tftp_server")
		return
	}
	var (
		ipAddress       string = os.Args[1]
		username        string = os.Args[2]
		password        string = os.Args[3]
		enable_password string = os.Args[4]
		tftp_server     string = os.Args[5]
	)
	var ip = flag.String("ip", ipAddress, "location of the switch to manage")
	var userName = flag.String("userName", username, "username to connect to switch")
	var normalPw = flag.String("normalPW", password, "the standard switch ssh password")
	var enablePw = flag.String("enablePW", enable_password, "the enable password for esculated priv")
	var tftpServer = flag.String("tftpServer", tftp_server, "the tftp server ip address")
*/

	var ip = flag.String("ip", "192.168.1.3", "location of the switch to manage")
	var userName = flag.String("userName", "kayer", "username to connect to switch")
	var normalPw = flag.String("normalPW", "kayer", "the standard switch ssh password")
	var enablePw = flag.String("enablePW", "kayer", "the enable password for esculated priv")
	var tftpServer = flag.String("tftpServer", "192.168.1.66", "the tftp server ip address")

	flag.Parse()

	fmt.Println("IP Chosen: ", *ip)
	fmt.Println("Username", *userName)
	fmt.Println("Normal PW", *normalPw)
	fmt.Println("Enable PW", *enablePw)
	fmt.Println("TFTP Server", *tftpServer)

	sshConfig := &ssh.ClientConfig{
		User: *userName,
		Auth: []ssh.AuthMethod{
			ssh.Password(*normalPw),
		},
		HostKeyCallback: func(ipAddress string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	sshConfig.Config.Ciphers = append(sshConfig.Config.Ciphers, "aes128-cbc")
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	connection, err := ssh.Dial("tcp", *ip+":22", sshConfig)
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}
	session, err := connection.NewSession()
	handleError(err, true, "Failed to create session: %s")

	sshOut, err := session.StdoutPipe()
	handleError(err, true, "Unable to setup stdin for session: %v")
	sshIn, err := session.StdinPipe()
	handleError(err, true, "Unable to setup stdout for session: %v")
	if err := session.RequestPty("xterm", 0, 200, modes); err != nil {
		session.Close()
		handleError(err, true, "request for pseudo terminal failed: %s")
	}
	if err := session.Shell(); err != nil {
		session.Close()
		handleError(err, true, "request for shell failed: %s")
	}
	readBuff(">", sshOut, 2)
	if _, err := writeBuff("enable", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	if _, err := writeBuff(*enablePw, sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	if _, err := writeBuff("show running", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	if _, err := writeBuff("\r", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	if _, err := writeBuff("copy running-config tftp", sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	readBuff("Address or name of remote host []?", sshOut, 2)
	if _, err := writeBuff(*tftpServer, sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	readBuff("confg]?", sshOut, 2)
	filename := []string{"switchBackup-", strings.Replace(*ip, ".", "-", -1), "_", strings.Replace(time.Now().Format(time.RFC3339), ":", "", -1)}
	if _, err := writeBuff(strings.Join(filename, ""), sshIn); err != nil {
		handleError(err, true, "Failed to run: %s")
	}
	fmt.Println(readBuff("bytes/sec)", sshOut, 60))
	session.Close()
}
