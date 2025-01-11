package ws

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToServers(dns_seeds []config.Dns_seed_config) {
	for _, dns_seed := range dns_seeds {
		conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
		if err != nil {
			logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
			continue
		}

		cm.mutex.Lock()
		cm.servers[dns_seed.UID] = conn
		cm.mutex.Unlock()
		logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)
	}
}

// ConnectToServers connects to all servers listed in the configs and stores servers with their UID.
func (cm *ClientManager) ConnectToOneServer(dns_seed config.Dns_seed_config) bool {

	conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
	if err != nil {
		logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
	}

	cm.mutex.Lock()
	cm.servers[dns_seed.UID] = conn
	cm.mutex.Unlock()
	logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)

	return true
}

// DisconnectFromServers disconnects from all servers.
func (cm *ClientManager) DisconnectFromServers() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for uid, conn := range cm.servers {
		conn.Close()
		logger.WS_S_LOGGER.Printf("Disconnected from server %v", uid)
		delete(cm.servers, uid)
	}
}

func MonitorAndReconnectToServers(cm *ClientManager) {
	for {
		time.Sleep(config.RECONNECT_SLEEP_TIME * time.Second) // Wait for 5 seconds

		cm.mutex.Lock()

		for _, dns_seed := range config.DNS_SEED_LIST {
			if cm.servers[dns_seed.UID] == nil { // If the server is disconnected
				logger.WS_F_LOGGER.Printf("Attempting to reconnect to server: %v", dns_seed.Address)

				conn, _, err := websocket.DefaultDialer.Dial(dns_seed.Address, nil)
				if err != nil {
					logger.WS_F_LOGGER.Printf("Failed to connect to server %s (%s): %v\n", dns_seed.UID, dns_seed.Address, err)
				} else {
					cm.servers[dns_seed.UID] = conn
					logger.WS_S_LOGGER.Printf("Connected to server %s (%s)\n", dns_seed.UID, dns_seed.Address)
				}
			}
		}

		cm.mutex.Unlock()
	}
}
