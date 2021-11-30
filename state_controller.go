package main

import "github.com/DisgoOrg/disgo/oauth2"

var _ oauth2.StateController = (*CustomStateController)(nil)

func NewCustomStateController() oauth2.StateController {
	return &stateControllerImpl{states: map[string]string{}}
}

type CustomStateController struct {
	states map[string]string
}


func (c *CustomStateController) GenerateNewState(redirectURI string) string {
	state := insecurerandstr.RandStr(32)
	c.states[state] = redirectURI
	return state
}

func (c *CustomStateController) ConsumeState(state string) *string {
	uri, ok := c.states[state]
	if !ok {
		return nil
	}
	delete(c.states, state)
	return &uri
}