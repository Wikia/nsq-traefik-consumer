// Copyright © 2017 Wikia Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/Wikia/nsq-traefik-consumer/queue"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "starts consuming messages",
	Long:  `Connects to a NSQ queue and starts processing messages.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := common.NewConfig()
		err := viper.Unmarshal(&config)
		if err != nil {
			common.Log.WithError(err).Errorf("Error parsing config")
		}
		buffer := queue.NewMetricsBuffer()
		go common.ServeStats()

		queue.RunSender(config.InfluxDB, buffer)
		queue.Consume(config, buffer)
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
}
