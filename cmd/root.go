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
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/Wikia/nsq-traefik-consumer/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "nsq-traefik-consumer",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		common.Log.WithError(err).Errorf("Error running command")
		os.Exit(-1)
	}
}

func init() {
	viper.SetDefault("LogLevel", "info")
	viper.SetDefault("LogAsJson", true)
	viper.SetDefault("BatchSize", 100)
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nsq-traefik-consumer.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigName(".nsq-traefik-consumer") // name of config file (without extension)
	viper.AddConfigPath("$HOME")                 // adding home directory as first search path
	viper.AddConfigPath(".")                     // adding home directory as first search path
	viper.AutomaticEnv()                         // read in environment variables that match

	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		common.Log.WithError(err).Error("Error loading config file")
	} else {
		common.Log.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	if viper.GetBool("LogAsJson") {
		log.SetFormatter(&log.JSONFormatter{})
	}

	level, err := log.ParseLevel(viper.GetString("LogLevel"))

	if err != nil {
		common.Log.WithError(err).Error("Error parsing LogLevel - setting to Info")
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}
}
