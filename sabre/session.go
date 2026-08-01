package sabre

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
)

const (
	sessionCreateXML = `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
  <SOAP-ENV:Header>
    <MessageHeader xmlns="http://www.ebxml.org/namespaces/messageHeader">
      <From><PartyId>Agency</PartyId></From>
      <To><PartyId>Sabre_API</PartyId></To>
      <ConversationId>%s</ConversationId>
      <Action>SessionCreateRQ</Action>
    </MessageHeader>
    <Security xmlns="http://schemas.xmlsoap.org/ws/2002/12/secext">
      <UsernameToken>
        <Username>%s</Username>
        <Password>%s</Password>
        <Organization>%s</Organization>
        <Domain>%s</Domain>
				<ClientId>5B0K-JvBdOta</ClientId>
				<ClientSecret>M1uty91x</ClientSecret>
      </UsernameToken>
    </Security>
  </SOAP-ENV:Header>
  <SOAP-ENV:Body>
    <SessionCreateRQ returnContextID="true" Version="1.0.0" xmlns="http://www.opentravel.org/OTA/2002/11"/>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

	sessionCloseXML = `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
  <SOAP-ENV:Header>
    <MessageHeader xmlns="http://www.ebxml.org/namespaces/messageHeader">
      <From><PartyId>Agency</PartyId></From>
      <To><PartyId>Sabre_API</PartyId></To>
      <ConversationId>%s</ConversationId>
      <Action>SessionCloseRQ</Action>
    </MessageHeader>
    <Security xmlns="http://schemas.xmlsoap.org/ws/2002/12/secext">
      <BinarySecurityToken valueType="String" EncodingType="wsse:Base64Binary">%s</BinarySecurityToken>
    </Security>
  </SOAP-ENV:Header>
  <SOAP-ENV:Body>
    <SessionCloseRQ Version="1.0.0" xmlns="http://www.opentravel.org/OTA/2002/11"/>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`
)

type SessionResult struct {
	ConversationID string
	Token          string
}

func CreateSession(endpoint, username, password, pcc, domain string) (*SessionResult, error) {
	conversationID := fmt.Sprintf("Go-%d", makeTimestamp())

	xmlBody := fmt.Sprintf(
		sessionCreateXML,
		conversationID,
		xmlEscape(username),
		xmlEscape(password),
		xmlEscape(pcc),
		xmlEscape(domain),
	)

	resp, err := http.Post(endpoint, "text/xml", strings.NewReader(xmlBody))
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sabre http error: %d\n%s", resp.StatusCode, string(body))
	}

	token, err := extractBinarySecurityToken(string(body))
	if err != nil {
		return nil, fmt.Errorf("extract token failed: %w\nResponse:\n%s", err, string(body))
	}

	return &SessionResult{
		ConversationID: conversationID,
		Token:          token,
	}, nil
}

func CloseSession(endpoint, conversationID, token string) error {
	xmlBody := fmt.Sprintf(
		sessionCloseXML,
		xmlEscape(conversationID),
		xmlEscape(token),
	)

	resp, err := http.Post(endpoint, "text/xml", strings.NewReader(xmlBody))
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sabre close http error: %d\n%s", resp.StatusCode, string(body))
	}

	doc, err := xmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("parse close response failed: %w", err)
	}

	node := xmlquery.FindOne(doc, "//*[local-name()='SessionCloseRS']")
	if node == nil {
		return fmt.Errorf("SessionCloseRS not found in response")
	}

	status := ""
	for _, attr := range node.Attr {
		if attr.Name.Local == "status" {
			status = attr.Value
			break
		}
	}

	if strings.ToLower(status) != "approved" {
		return fmt.Errorf("session close not approved, status: %s", status)
	}

	return nil
}

func extractBinarySecurityToken(xmlBody string) (string, error) {
	doc, err := xmlquery.Parse(strings.NewReader(xmlBody))
	if err != nil {
		return "", fmt.Errorf("parse xml failed: %w", err)
	}

	node := xmlquery.FindOne(doc, "//*[local-name()='BinarySecurityToken']")
	if node == nil {
		return "", fmt.Errorf("BinarySecurityToken not found")
	}

	return strings.TrimSpace(node.InnerText()), nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func makeTimestamp() int64 {
	return time.Now().UnixNano()
}
