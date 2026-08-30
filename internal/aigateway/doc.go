// Package aigateway embeds Bifrost as an in-process LLM gateway.
// Real provider keys stay in Account; Judge and OpenCode talk OpenAI chat
// completions only via a loopback listener.
package aigateway
