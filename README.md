# smtplmail

smtplmail is a drop-in replacement for sendmail, designed to send emails from Linux systems using SMTP. It provides a simple way to configure and use SMTP for outgoing mail, making it easy to integrate with existing systems that expect sendmail functionality.

## Features

- Easy setup and configuration
- Support for SSL, TLS, and unencrypted connections
- Configurable via YAML file, environment variables, or command-line flags
- Logging support with configurable log levels
- Sendmail-compatible command-line interface

## Installation

1. Download the latest release binary from the [Releases](https://github.com/teamcoltra/smtplmail/releases) page.

2. Run the setup process as root:

   ```
   sudo ./smtplmail --setup
   ```

   This will:
   - Prompt you for SMTP configuration details
   - Create the configuration file at `/etc/smtplemail/smtplemail.conf`
   - Move the binary to `/usr/local/bin/smtplmail`
   - Create a symlink at `/usr/bin/sendmail` pointing to the smtplmail binary

## Configuration

The configuration file is located at `/etc/smtplemail/smtplemail.conf`. Here's an example configuration:

```yaml
smtp_user: your_username
smtp_password: your_password
smtp_host: smtp.example.com
smtp_port: "587"
smtp_security: TLS
send_from: your_email@example.com
log_file: /var/log/smtplemail/smtplemail.log
log_level: error
```

You can also set these values using environment variables or command-line flags.

## Usage

smtplmail can be used as a drop-in replacement for sendmail. It accepts email content via stdin and supports various command-line flags:

```
smtplmail [options] recipient1 [recipient2 ...]
```

### Command-line Options

- `-f`: Sets the envelope sender address
- `-F`: Sets the sender's full name
- `-N`: Specifies delivery status notifications (NEVER, SUCCESS, FAILURE, DELAY)
- `-v`: Enables verbose mode for detailed output
- `-i`: Ignores single dots on a line
- `-t`: Reads recipients from the message headers (To, Cc, Bcc)

### Configuration Flags

These flags can be used to override the configuration file settings:

- `--config`: Specify an alternate configuration file location
- `--smtp_user`: SMTP username
- `--smtp_password`: SMTP password
- `--smtp_host`: SMTP server hostname
- `--smtp_port`: SMTP server port
- `--smtp_security`: SMTP security (SSL, TLS, or None)
- `--send_from`: Default "From" email address
- `--log_file`: Log file location
- `--log_level`: Log level (error or info)

## Examples

Send an email using the configuration file settings:

```
echo "Subject: Test Email
This is a test email." | smtplmail recipient@example.com
```

Send an email with a custom sender:

```
echo "Subject: Test Email
This is a test email." | smtplmail -f sender@example.com recipient@example.com
```

Send an email using recipients from headers:

```
echo "To: recipient1@example.com
Cc: recipient2@example.com
Subject: Test Email

This is a test email." | smtplmail -t
```

## Logging

Logs are written to the file specified in the configuration (default: `/var/log/smtplemail/smtplemail.log`). The log level can be set to "error" or "info" in the configuration file or using the `--log_level` flag.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. I am still learning Go so I welcome help and advice on best practices. One of the things I was going to do is split the config and mail sending into separate files for better management.

## License

Released under the MIT license