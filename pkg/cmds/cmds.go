package cmds

import (
	"github.com/lukeshay/gocden/pkg/config"
	cli "github.com/urfave/cli/v2"
)

func GetConfigFromCliContext(c *cli.Context) *config.Config {
	return c.Context.Value(config.ConfigPath).(*config.Config)
}

func GetCwdFlag(c *cli.Context) string {
	return c.String("cwd")
}
