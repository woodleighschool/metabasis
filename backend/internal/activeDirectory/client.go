package activedirectory

import (
	"fmt"

	"github.com/dlampsi/adc"
	"github.com/woodleighschool/adoverseas/internal/config"
)

type Client struct {
	Client     *adc.Client
	AwayGroups []string
	HomeGroups []string
	MFAGroup   string
}

func NewClient(cfg config.Config) (Client, error) {
	adcCfg := &adc.Config{
		URL: fmt.Sprintf("ldap://%s:389", cfg.ADHost),
		Bind: &adc.BindAccount{
			DN:       cfg.ADAdminDN,
			Password: cfg.ADAdminPassword,
		},
		SearchBase: cfg.ADBase,
	}

	cl := adc.New(adcCfg)

	if err := cl.Connect(); err != nil {
		return Client{}, fmt.Errorf("unable to create AD client: %w", err)
	} else {
		return Client{Client: cl, AwayGroups: cfg.AwayGroups, HomeGroups: cfg.HomeGroups, MFAGroup: cfg.MFAGroup}, nil
	}
}
