package sabre

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/antchfx/xmlquery"
)

const commandXML = `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/">
  <SOAP-ENV:Header>
    <MessageHeader xmlns="http://www.ebxml.org/namespaces/messageHeader">
      <From><PartyId>SWS</PartyId></From>
      <To><PartyId>Agency</PartyId></To>
      <ConversationId>%s</ConversationId>
      <Action>SabreCommandLLSRQ</Action>
    </MessageHeader>
    <Security xmlns="http://schemas.xmlsoap.org/ws/2002/12/secext">
      <BinarySecurityToken valueType="String" EncodingType="wsse:Base64Binary">%s</BinarySecurityToken>
    </Security>
  </SOAP-ENV:Header>
  <SOAP-ENV:Body>
    <SabreCommandLLSRQ ReturnHostCommand="true" Version="2.0.0" xmlns="http://webservices.sabre.com/sabreXML/2011/10">
      <Request Output="SCREEN">
        <HostCommand>%s</HostCommand>
      </Request>
    </SabreCommandLLSRQ>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

type CommandResult struct {
	Success        bool
	Status         string
	Response       string
	Errors         []string
	SessionRetried bool
}

func SendCommand(endpoint, conversationID, token, command string) (*CommandResult, error) {
	xmlBody := fmt.Sprintf(
		commandXML,
		xmlEscape(conversationID),
		xmlEscape(token),
		xmlEscape(command),
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
		return nil, fmt.Errorf("sabre command http error: %d\n%s", resp.StatusCode, string(body))
	}

	return parseCommandResponse(string(body))
}

func SendCommandWithExistingSession(cfg *Config, ses *SessionResult, command string) (*CommandResult, error) {
	result, err := SendCommand(cfg.Endpoint, ses.ConversationID, ses.Token, command)
	if err != nil {
		return nil, err
	}

	if isSessionInvalid(result) {
		newSes, err2 := CreateSession(
			cfg.Endpoint,
			cfg.Username,
			cfg.Password,
			cfg.PCC,
			cfg.Domain,
			cfg.IsLive,
		)
		if err2 != nil {
			return nil, fmt.Errorf("session expired and retry session creation failed: %w", err2)
		}

		result2, err2 := SendCommand(cfg.Endpoint, newSes.ConversationID, newSes.Token, command)
		if err2 != nil {
			return nil, err2
		}

		ses.ConversationID = newSes.ConversationID
		ses.Token = newSes.Token
		result2.SessionRetried = true
		return result2, nil
	}

	return result, nil
}

func parseCommandResponse(xmlBody string) (*CommandResult, error) {
	doc, err := xmlquery.Parse(strings.NewReader(xmlBody))
	if err != nil {
		return &CommandResult{
			Success: false,
			Errors:  []string{"Unable to parse Sabre XML response."},
		}, nil
	}

	result := &CommandResult{}

	appResults := xmlquery.FindOne(doc, "//*[local-name()='ApplicationResults']")
	if appResults != nil {
		for _, attr := range appResults.Attr {
			if attr.Name.Local == "status" {
				result.Status = attr.Value
				break
			}
		}
	}

	responseNode := xmlquery.FindOne(doc, "//*[local-name()='Response']")
	if responseNode != nil {
		result.Response = strings.TrimSpace(responseNode.InnerText())
	}

	errorNodes := xmlquery.Find(doc, "//*[local-name()='Error']")
	for _, node := range errorNodes {
		result.Errors = append(result.Errors, strings.TrimSpace(node.InnerText()))
	}

	result.Success = result.Status == "Complete"

	return result, nil
}

func isSessionInvalid(result *CommandResult) bool {
	text := strings.ToLower(
		fmt.Sprintf("%s %s %s",
			result.Status,
			result.Response,
			strings.Join(result.Errors, " "),
		),
	)

	keywords := []string{
		"session",
		"security token",
		"binarysecuritytoken",
		"invalid session",
		"session expired",
		"session not found",
		"not authorized",
		"unauthorized",
	}

	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
