package logs

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/smtp"
	"strings"
	"fmt"
)

type SMTPWriter struct {
	Username		string	`json:"username"`
	Password		string	`json:"password"`
	Host				string	`json:"host"`
	Subject			string	`json:"subject"`
	FromAddress	string	`json:"fromAddress"`
	ToAddresses		[]string	`json:"toAddress"`
	Level  			int  		`json:"level"`
}

// NewSMTPWriter creates smtp writer
func newSMTPWriter() Logger {
	return &SMTPWriter{Level: LevelDebug}
}

func (c *SMTPWriter) SetFormatter(f LogFormatter) {}

// Init smtp writer with json config.
// config like:
//	{
//		"username":"example@gmail.com",
//		"password:"password",
//		"host":"smtp.gmail.com:465",
//		"subject":"email title",
//		"fromAddress":"from@example.com",
//		"sendTos":["email1","email2"],
//		"level":LevelError
//	}
func (this *SMTPWriter) Init(jsonConfig string) error {
	return json.Unmarshal([]byte(jsonConfig), this)
}

func (this *SMTPWriter) getSMTPAuth(host string) smtp.Auth {
	if len(strings.Trim(this.Username, " ")) == 0 && len(strings.Trim(this.Password, " ")) == 0 {
		return nil
	}
	return smtp.PlainAuth(
		"",
		this.Username,
		this.Password,
		host,
	)
}

func (s *SMTPWriter) sendMail(hostAddressWithPort string, auth smtp.Auth, fromAddress string, recipients []string, msgContent []byte) error {
	client, err := smtp.Dial(hostAddressWithPort)
	if err != nil {
		return err
	}

	host, _, _ := net.SplitHostPort(hostAddressWithPort)
	tlsConn := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	if err = client.StartTLS(tlsConn); err != nil {
		return err
	}

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	if err = client.Mail(fromAddress); err != nil {
		return err
	}

	for _, rec := range recipients {
		if err = client.Rcpt(rec); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msgContent)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// WriteMsg write message in smtp writer.
// it will send an email with subject and only this message.
func (s *SMTPWriter) WriteMsg(lm *LogMsg) error {
	if lm.Level > s.Level {
		return nil
	}

	when := lm.When
	msg := lm.Msg

	hp := strings.Split(s.Host, ":")

	// Set up authentication information.
	auth := s.getSMTPAuth(hp[0])

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	mailmsg := []byte("To: " + strings.Join(s.ToAddresses, ";") + "\r\nFrom: " + s.FromAddress + "<" + s.FromAddress +
		">\r\nSubject: " + s.Subject + "\r\n" + contentType + "\r\n\r\n" + fmt.Sprintf(".%s", when.Format("2006-01-02 15:04:05")) + msg)

	return s.sendMail(s.Host, auth, s.FromAddress, s.ToAddresses, mailmsg)
}

// Flush implementing method. empty.
func (s *SMTPWriter) Flush() {
}

// Destroy implementing method. empty.
func (s *SMTPWriter) Destroy() {
}

func init() {
	Register(AdapterMail, newSMTPWriter)
}
