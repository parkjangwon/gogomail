package imapgw

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

func TestThunderbirdBatchFlagFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	// Step 1: LOGIN
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	// Step 2: SELECT inbox
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	// Step 3: UID FETCH FLAGS (Thunderbird pattern)
	if _, err := client.Write([]byte("a3 UID FETCH 7:* FLAGS\r\n")); err != nil {
		t.Fatalf("write thunderbird batch flags fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read flags response: %v", err)
		}
		if line != expected {
			t.Fatalf("flags response = %q, want %q", line, expected)
		}
	}
	// Step 4: LOGOUT
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestThunderbirdUIDFetchWithSequence(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 BODY.PEEK[HEADER.FIELDS (FROM TO CC SUBJECT DATE)]\r\n")); err != nil {
		t.Fatalf("write thunderbird header fields fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 BODY[HEADER.FIELDS (FROM TO CC SUBJECT DATE)] {2}\r\n",
		"\r\n",
		")\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read header fields response: %v", err)
		}
		if line != expected {
			t.Fatalf("header fields response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailFetchBodyStructureAndUID(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (UID BODY[HEADER] BODY[TEXT])\r\n")); err != nil {
		t.Fatalf("write apple mail fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11 BODY[HEADER] {11}\r\n",
		"hello world)\r\n",
		"a3 OK UID FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		if line != expected {
			t.Fatalf("Apple Mail response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailMultiFetchSequence(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:2 (FLAGS UID RFC822.SIZE)\r\n")); err != nil {
		t.Fatalf("write apple mail multi fetch: %v", err)
	}
	want := []string{
		"* 1 FETCH (UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 11)\r\n",
		"* 2 FETCH (UID 8 FLAGS (\\Seen \\Flagged) RFC822.SIZE 41)\r\n",
		"a3 OK FETCH completed\r\n",
	}
	for _, expected := range want {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read multi fetch response: %v", err)
		}
		if line != expected {
			t.Fatalf("Apple Mail multi fetch response = %q, want %q", line, expected)
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailFetchBodyPeek(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	// login response
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	// 7 select response lines
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}

	// K-9 Mail uses UID FETCH with BODY.PEEK[TEXT] to fetch body without setting \Seen
	// UID 9: "Subject: Hello\r\nFrom: sender@test\r\n\r\nhello header body" — TEXT = "hello header body" (17 bytes)
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[TEXT]\r\n")); err != nil {
		t.Fatalf("write peek body: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read fetch literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[TEXT] {17}\r\n" {
		t.Fatalf("fetch literal header = %q", line)
	}
	body := make([]byte, 17)
	if _, err := io.ReadFull(reader, body); err != nil {
		t.Fatalf("read body literal: %v", err)
	}
	if string(body) != "hello header body" {
		t.Fatalf("body = %q", body)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("literal close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}

	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailUIDFetchBody(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}

	// K-9 Mail UID FETCH for body text (UID 9: "Subject: Hello\r\nFrom: sender@test\r\n\r\nhello header body")
	// TEXT part = "hello header body" (17 bytes)
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY[TEXT]\r\n")); err != nil {
		t.Fatalf("write uid fetch body: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read uid fetch literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[TEXT] {17}\r\n" {
		t.Fatalf("uid fetch literal header = %q", line)
	}
	textBody := make([]byte, 17)
	if _, err := io.ReadFull(reader, textBody); err != nil {
		t.Fatalf("read uid fetch literal: %v", err)
	}
	if string(textBody) != "hello header body" {
		t.Fatalf("text body = %q", textBody)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("literal close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}

	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestThunderbirdEnvelopeFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (ENVELOPE)\r\n")); err != nil {
		t.Fatalf("write thunderbird envelope fetch: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read envelope response: %v", err)
	}
	if !strings.Contains(line, "ENVELOPE") {
		t.Fatalf("Thunderbird envelope response missing ENVELOPE: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailInternalDateFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 INTERNALDATE RFC822.SIZE\r\n")); err != nil {
		t.Fatalf("write apple mail internal date fetch: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read internal date response: %v", err)
	}
	if !strings.Contains(line, "INTERNALDATE") {
		t.Fatalf("Apple Mail internal date response missing INTERNALDATE: %s", line)
	}
	if !strings.Contains(line, "RFC822.SIZE") {
		t.Fatalf("Apple Mail internal date response missing RFC822.SIZE: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailSequenceRangeFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:2 (FLAGS)\r\n")); err != nil {
		t.Fatalf("write k9 sequence range fetch: %v", err)
	}
	for i := 0; i < 4; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read range fetch response: %v", err)
		}
		if strings.HasPrefix(line, "a3") {
			break
		}
		if strings.Contains(line, "FETCH") {
			if !strings.Contains(line, "FLAGS") {
				t.Fatalf("K-9 Mail range fetch missing FLAGS: %s", line)
			}
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestThunderbirdBodystructureFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (BODYSTRUCTURE)\r\n")); err != nil {
		t.Fatalf("write thunderbird bodystructure fetch: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read bodystructure response: %v", err)
	}
	if !strings.Contains(line, "BODYSTRUCTURE") {
		t.Fatalf("Thunderbird bodystructure response missing BODYSTRUCTURE: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailPeekHeaderFields(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}

	// Apple Mail fetches headers via BODY.PEEK[HEADER.FIELDS (FROM TO SUBJECT DATE)]
	// UID 9: "Subject: Hello\r\nFrom: sender@test\r\n\r\nhello header body" (54 bytes)
	// Present headers: Subject (16 bytes) + From (20 bytes) + CRLF terminator (2) = 38 bytes
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[HEADER.FIELDS (FROM TO SUBJECT DATE)]\r\n")); err != nil {
		t.Fatalf("write apple header fields: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read header fields literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)] {37}\r\n" {
		t.Fatalf("header fields literal header = %q", line)
	}
	headers := make([]byte, 37)
	if _, err := io.ReadFull(reader, headers); err != nil {
		t.Fatalf("read header fields literal: %v", err)
	}
	if string(headers) != "Subject: Hello\r\nFrom: sender@test\r\n\r\n" {
		t.Fatalf("header fields = %q", headers)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("literal close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}

	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailGmailExtensionFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1:* (UID FLAGS)\r\n")); err != nil {
		t.Fatalf("write k9 gmail extension fetch: %v", err)
	}
	for i := 0; i < 4; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read gmail extension response: %v", err)
		}
		if strings.Contains(line, "FETCH") && strings.Contains(line, "UID") {
			if !strings.Contains(line, "FLAGS") {
				t.Fatalf("K-9 Mail gmail extension missing FLAGS: %s", line)
			}
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	reader.ReadString('\n')
	reader.ReadString('\n')
	<-errCh
}

func TestThunderbirdUIDFetchEnvelopeAndFlags(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (UID FLAGS ENVELOPE)\r\n")); err != nil {
		t.Fatalf("write thunderbird uid fetch envelope and flags: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read uid fetch response: %v", err)
	}
	if !strings.Contains(line, "UID") || !strings.Contains(line, "FLAGS") || !strings.Contains(line, "ENVELOPE") {
		t.Fatalf("Thunderbird uid fetch missing UID, FLAGS, or ENVELOPE: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailRFC822SizeFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7 (UID RFC822.SIZE FLAGS)\r\n")); err != nil {
		t.Fatalf("write apple mail rfc822size fetch: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read rfc822size response: %v", err)
	}
	if !strings.Contains(line, "RFC822.SIZE") {
		t.Fatalf("Apple Mail rfc822size response missing RFC822.SIZE: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailMessageSequenceFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID SEARCH RETURN (MIN MAX)\r\n")); err != nil {
		t.Fatalf("write k9 uid search: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read uid search response: %v", err)
	}
	if !strings.Contains(line, "SEARCH") && !strings.Contains(line, "MIN") && !strings.Contains(line, "MAX") {
		t.Fatalf("K-9 Mail uid search response unexpected: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestThunderbirdMultipleUIDFetch(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 UID FETCH 7,9 (FLAGS UID)\r\n")); err != nil {
		t.Fatalf("write thunderbird multiple uid fetch: %v", err)
	}
	for i := 0; i < 3; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read multiple uid response: %v", err)
		}
		if strings.HasPrefix(line, "a3") {
			break
		}
		if strings.Contains(line, "FETCH") {
			if !strings.Contains(line, "UID") || !strings.Contains(line, "FLAGS") {
				t.Fatalf("Thunderbird multiple uid missing UID or FLAGS: %s", line)
			}
		}
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestAppleMailMIMELoading(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\n")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	if _, err := client.Write([]byte("a2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write select: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}
	if _, err := client.Write([]byte("a3 FETCH 1 BODY[1.MIME]\r\n")); err != nil {
		t.Fatalf("write apple mail mime loading: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read mime loading response: %v", err)
	}
	if !strings.Contains(line, "BODY[1.MIME]") && !strings.Contains(line, "BAD") && !strings.Contains(line, "NO") {
		t.Fatalf("Apple Mail mime loading response unexpected: %s", line)
	}
	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}

func TestK9MailPartialWithOffset(t *testing.T) {
	t.Parallel()

	server, err := NewServer(ServerOptions{Addr: ":1143", Backend: fakeBackend{}, AllowInsecureAuth: true})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	client, backend := net.Pipe()
	defer client.Close()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeConn(backend)
	}()

	reader := bufio.NewReader(client)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if _, err := client.Write([]byte("a1 LOGIN user@example.com secret\r\na2 SELECT inbox\r\n")); err != nil {
		t.Fatalf("write login/select: %v", err)
	}
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read login response: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read select response: %v", err)
		}
	}

	// K-9 Mail partial fetch: BODY.PEEK[TEXT]<6.6> on UID 9
	// TEXT = "hello header body" (17 bytes), offset 6, max 6 bytes → "header"
	if _, err := client.Write([]byte("a3 UID FETCH 9 BODY.PEEK[TEXT]<6.6>\r\n")); err != nil {
		t.Fatalf("write partial fetch: %v", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read partial fetch literal header: %v", err)
	}
	if line != "* 3 FETCH (UID 9 FLAGS (\\Seen \\Flagged) RFC822.SIZE 54 BODY[TEXT]<6> {6}\r\n" {
		t.Fatalf("partial fetch literal header = %q", line)
	}
	partialBody := make([]byte, 6)
	if _, err := io.ReadFull(reader, partialBody); err != nil {
		t.Fatalf("read partial fetch literal: %v", err)
	}
	if string(partialBody) != "header" {
		t.Fatalf("partial body = %q", partialBody)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != ")\r\n" {
		t.Fatalf("literal close = %q err = %v", line, err)
	}
	if line, err = reader.ReadString('\n'); err != nil || line != "a3 OK UID FETCH completed\r\n" {
		t.Fatalf("completion = %q err = %v", line, err)
	}

	if _, err := client.Write([]byte("a4 LOGOUT\r\n")); err != nil {
		t.Fatalf("write logout: %v", err)
	}
	_, _ = reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	<-errCh
}