package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config holds the SMTP configuration
type Config struct {
	SMTPUser     string `yaml:"smtp_user"`
	SMTPPassword string `yaml:"smtp_password"`
	SMTPHost     string `yaml:"smtp_host"`
	SMTPPort     string `yaml:"smtp_port"`
	SMTPSecurity string `yaml:"smtp_security"` // Can be "SSL", "TLS", or "None"
	SendFrom     string `yaml:"send_from"`
	LogFile      string `yaml:"log_file"`
	LogLevel     string `yaml:"log_level"` // Can be "error" or "info"
}

var configFile string
var config Config
var setup bool
var recipients []string
var sender string
var senderFullName string
var deliveryStatusNotification string
var verbose bool
var ignoreDots bool
var readRecipientsFromHeaders bool

func init() {
	// Set default configuration file location
	flag.StringVar(&configFile, "config", "/etc/smtplemail/smtplemail.conf", "Configuration file location")
	flag.BoolVar(&setup, "setup", false, "Run setup to create config file and setup symlink")

	// Sendmail flags
	flag.StringVar(&sender, "f", "", "Sets the envelope sender address")
	flag.StringVar(&senderFullName, "F", "", "Sets the sender's full name")
	flag.StringVar(&deliveryStatusNotification, "N", "", "Specifies delivery status notifications (NEVER, SUCCESS, FAILURE, DELAY)")
	flag.BoolVar(&verbose, "v", false, "Enables verbose mode for detailed output")
	flag.BoolVar(&ignoreDots, "i", false, "Ignores single dots on a line")
	flag.BoolVar(&readRecipientsFromHeaders, "t", false, "Reads recipients from the message headers (To, Cc, Bcc)")

	// Load configuration from environment variables
	loadConfigFromEnv()

	// Load configuration from command-line flags
	flag.StringVar(&config.SMTPUser, "smtp_user", config.SMTPUser, "SMTP user")
	flag.StringVar(&config.SMTPPassword, "smtp_password", config.SMTPPassword, "SMTP password")
	flag.StringVar(&config.SMTPHost, "smtp_host", config.SMTPHost, "SMTP host")
	flag.StringVar(&config.SMTPPort, "smtp_port", config.SMTPPort, "SMTP port")
	flag.StringVar(&config.SMTPSecurity, "smtp_security", config.SMTPSecurity, "SMTP security (SSL, TLS, or None)")
	flag.StringVar(&config.SendFrom, "send_from", config.SendFrom, "Send email from this address")
	flag.StringVar(&config.LogFile, "log_file", "/var/log/smtplemail/smtplemail.log", "Log file location")
	flag.StringVar(&config.LogLevel, "log_level", "error", "Log level (error or info)")

	flag.Parse()

	recipients = flag.Args()
}

func main() {
	if setup {
		runSetup()
		return
	}

	// Load configuration from file if specified
	loadConfigFromFile(configFile)

	// Set up logging
	setupLogging()

	// Read email data from stdin (sendmail behavior)
	reader := bufio.NewReader(os.Stdin)
	message, err := io.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to read email data: %v", err)
	}

	if readRecipientsFromHeaders {
		recipients = extractRecipientsFromHeaders(string(message))
	}

	if len(recipients) == 0 {
		log.Fatalf("No recipients specified")
	}

	// Send the email
	err = sendEmail(recipients, string(message))
	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}
}

func runSetup() {
	reader := bufio.NewReader(os.Stdin)

	// Gather configuration details
	fmt.Print("Enter SMTP user: ")
	config.SMTPUser, _ = reader.ReadString('\n')
	config.SMTPUser = strings.TrimSpace(config.SMTPUser)

	fmt.Print("Enter SMTP password: ")
	config.SMTPPassword, _ = reader.ReadString('\n')
	config.SMTPPassword = strings.TrimSpace(config.SMTPPassword)

	fmt.Print("Enter SMTP host: ")
	config.SMTPHost, _ = reader.ReadString('\n')
	config.SMTPHost = strings.TrimSpace(config.SMTPHost)

	fmt.Print("Enter SMTP port: ")
	config.SMTPPort, _ = reader.ReadString('\n')
	config.SMTPPort = strings.TrimSpace(config.SMTPPort)

	fmt.Print("Use SSL, TLS, or None (SSL/TLS/None): ")
	config.SMTPSecurity, _ = reader.ReadString('\n')
	config.SMTPSecurity = strings.TrimSpace(config.SMTPSecurity)

	fmt.Print("Send email from (optional): ")
	config.SendFrom, _ = reader.ReadString('\n')
	config.SendFrom = strings.TrimSpace(config.SendFrom)

	fmt.Print("Log file location (default: /var/log/smtplemail/smtplemail.log): ")
	config.LogFile, _ = reader.ReadString('\n')
	config.LogFile = strings.TrimSpace(config.LogFile)
	if config.LogFile == "" {
		config.LogFile = "/var/log/smtplemail/smtplemail.log"
	}

	fmt.Print("Log level (error or info, default: error): ")
	config.LogLevel, _ = reader.ReadString('\n')
	config.LogLevel = strings.TrimSpace(config.LogLevel)
	if config.LogLevel == "" {
		config.LogLevel = "error"
	}

	// Save configuration to file
	saveConfigToFile(configFile)

	// Check for existing sendmail
	sendmailPath, err := exec.LookPath("sendmail")
	if err == nil {
		fmt.Printf("Existing sendmail binary found at %s. Rename to sendmail-old? (yes/no): ", sendmailPath)
		rename, _ := reader.ReadString('\n')
		if strings.TrimSpace(rename) == "yes" {
			err := os.Rename(sendmailPath, sendmailPath+"-old")
			if err != nil {
				log.Fatalf("Failed to rename existing sendmail: %v", err)
			}
		}
	}

	// Move binary to /usr/local/bin and create symlink
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	destPath := "/usr/local/bin/smtplmail"
	err = os.Rename(exePath, destPath)
	if err != nil {
		log.Fatalf("Failed to move executable: %v", err)
	}

	// Create symlink
	err = os.Symlink(destPath, "/usr/bin/sendmail")
	if err != nil {
		log.Fatalf("Failed to create symlink: %v", err)
	}

	fmt.Println("Setup completed successfully.")
}

func loadConfigFromFile(filename string) {
	data, err := os.ReadFile(filename)
	if err == nil {
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Fatalf("Failed to parse configuration file: %v", err)
		}
	}
}

func saveConfigToFile(filename string) {
	data, err := yaml.Marshal(&config)
	if err != nil {
		log.Fatalf("Failed to serialize configuration: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		log.Fatalf("Failed to create configuration directory: %v", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Fatalf("Failed to write configuration file: %v", err)
	}

	fmt.Printf("Configuration saved to %s\n", filename)
}

func loadConfigFromEnv() {
	if user, exists := os.LookupEnv("SMTP_USER"); exists {
		config.SMTPUser = user
	}
	if password, exists := os.LookupEnv("SMTP_PASSWORD"); exists {
		config.SMTPPassword = password
	}
	if host, exists := os.LookupEnv("SMTP_HOST"); exists {
		config.SMTPHost = host
	}
	if port, exists := os.LookupEnv("SMTP_PORT"); exists {
		config.SMTPPort = port
	}
	if security, exists := os.LookupEnv("SMTP_SECURITY"); exists {
		config.SMTPSecurity = security
	}
	if sendFrom, exists := os.LookupEnv("SEND_FROM"); exists {
		config.SendFrom = sendFrom
	}
	if logFile, exists := os.LookupEnv("LOG_FILE"); exists {
		config.LogFile = logFile
	}
	if logLevel, exists := os.LookupEnv("LOG_LEVEL"); exists {
		config.LogLevel = logLevel
	}
}

func extractRecipientsFromHeaders(message string) []string {
	var recipients []string
	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		log.Fatalf("Failed to parse email message: %v", err)
	}
	header := msg.Header
	for _, addr := range header["To"] {
		recipients = append(recipients, addr)
	}
	for _, addr := range header["Cc"] {
		recipients = append(recipients, addr)
	}
	for _, addr := range header["Bcc"] {
		recipients = append(recipients, addr)
	}
	return recipients
}

func parseEmailAddress(address string) (name string, email string, err error) {
	addr, err := mail.ParseAddress(address)
	if err != nil {
		return "", "", err
	}
	return addr.Name, addr.Address, nil
}

func formatEmailAddress(name, email string) string {
	if name == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func sendEmail(to []string, message string) error {
	msg, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to parse email message: %v", err)
	}

	from := config.SendFrom
	if from == "" {
		from = msg.Header.Get("From")
	}

	if from == "" {
		return fmt.Errorf("no From address specified in config or email headers")
	}

	// Parse the email address to separate display name and email
	name, email, err := parseEmailAddress(from)
	if err != nil {
		return fmt.Errorf("failed to parse From address: %v", err)
	}

	// Override with command-line arguments if provided
	if sender != "" {
		email = sender
	}
	if senderFullName != "" {
		name = senderFullName
	}

	// Reconstruct the From field
	from = formatEmailAddress(name, email)

	// Ensure the From header is in the email
	var newBody bytes.Buffer
	fromHeaderExists := false
	for k, vv := range msg.Header {
		if strings.EqualFold(k, "From") {
			fromHeaderExists = true
			fmt.Fprintf(&newBody, "From: %s\r\n", from)
		} else {
			for _, v := range vv {
				fmt.Fprintf(&newBody, "%s: %s\r\n", k, v)
			}
		}
	}
	if !fromHeaderExists {
		fmt.Fprintf(&newBody, "From: %s\r\n", from)
	}
	newBody.WriteString("\r\n")
	_, err = io.Copy(&newBody, msg.Body)
	if err != nil {
		return fmt.Errorf("failed to copy email body: %v", err)
	}
	message = newBody.String()

	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)
	addr := net.JoinHostPort(config.SMTPHost, config.SMTPPort)

	var conn net.Conn
	if config.SMTPSecurity == "SSL" {
		conn, err = tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %v", err)
	}

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}
	defer client.Close()

	if config.SMTPSecurity == "TLS" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			err = client.StartTLS(&tls.Config{ServerName: config.SMTPHost})
			if err != nil {
				return fmt.Errorf("failed to start TLS: %v", err)
			}
		}
	}

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	if err = client.Mail(email); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient: %v", err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get Data writer: %v", err)
	}

	_, err = wc.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = wc.Close()
	if err != nil {
		return fmt.Errorf("failed to close Data writer: %v", err)
	}

	if verbose {
		log.Printf("Email sent successfully to: %v\n", to)
	} else if config.LogLevel == "info" {
		log.Printf("Email sent successfully to: %v\n", to)
	}

	return nil
}

func setupLogging() {
	err := os.MkdirAll(filepath.Dir(config.LogFile), 0755)
	if err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	log.SetOutput(logFile)
	if config.LogLevel == "info" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}
}
