package basic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
)

const ScopeType = "basic"

func init() {
	scope.RegisterParser(&Parser{})
}

type Parser struct {
	cfg Config
}

type Config struct {
	TeamNameHeader string `json:"TeamNameHeader"`
}

func (p *Parser) GetName() string {
	return ScopeType
}

func (p *Parser) ParseNpaScope(message json.RawMessage) (scope.Scope, error) {
	return scope.FromJSON[Scope](message)
}

func (p *Parser) ParseUserHeader(c *gin.Context) []scope.Scope {
	teamsRaw := c.GetHeader(p.cfg.TeamNameHeader)
	if teamsRaw == "" {
		return nil
	}
	teams := strings.Split(teamsRaw, ",") // TODO: validate, but this is just an example scope anyway

	return []scope.Scope{
		&Scope{
			Actions:      []string{scope.Wildcard},
			Environments: []string{"dev", "tst"},
			Teams:        teams,
			Roles:        []string{"user"},
		},
		&Scope{
			Actions:      []string{scope.Wildcard},
			Environments: []string{"acc", "prd"},
			Teams:        teams,
			Roles:        []string{"user"},
		},
	}
}

// basic.Scope implements the scope.Scope interface
var _ scope.Scope = (*Scope)(nil)
var _ scope.Parser = (*Parser)(nil)

func NewScope(c *gin.Context, environment, team, role string) *Scope {
	return &Scope{
		Actions:      []string{c.Request.Method},
		Environments: []string{environment},
		Teams:        []string{team},
		Roles:        []string{role},
	}
}

type Scope struct { // The values of these fields depend on your organization
	Actions      []string `json:"actions"`      // E.g. GET, DELETE
	Environments []string `json:"environments"` // E.g. dev, tst, acc, prd
	Teams        []string `json:"teams"`        // E.g. foo, bar
	Roles        []string `json:"roles"`        // E.g. admin, user, auditor, etc.
}

func (s Scope) Validate() error {
	if len(s.Actions) == 0 {
		return errors.New("scope must have at least one action")
	}
	if len(s.Environments) == 0 {
		return errors.New("scope must have at least one environment")
	}
	if len(s.Teams) == 0 {
		return errors.New("scope must have at least one team")
	}
	if len(s.Roles) == 0 {
		return errors.New("scope must have at least one role")
	}
	return nil
}

func (s Scope) AsLabels() [][]string {
	allowedScopes := [][]string{}
	for _, team := range s.Teams {
		for _, environment := range s.Environments {
			for _, action := range s.Actions {
				if len(s.Roles) > 0 {
					for _, role := range s.Roles {
						allowedScopes = append(allowedScopes, []string{team, environment, action, role})
					}
				} else {
					allowedScopes = append(allowedScopes, []string{team, environment, action})
				}
			}
		}
	}
	return allowedScopes
}

func (s Scope) String() string {
	return fmt.Sprintf("{%v %v %v %v}", s.Actions, s.Environments, s.Teams, s.Roles)
}
