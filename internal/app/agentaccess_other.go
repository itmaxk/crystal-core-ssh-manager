//go:build !linux

package app

import "errors"

func agentSetupSupported() bool { return false }

func setupLocalAgentUser(name string, closeHome bool) (AgentSetupResult, error) {
	return AgentSetupResult{}, errors.New("настройка отдельного пользователя для ИИ-агента доступна только в Linux")
}
