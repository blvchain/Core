package config

type Delium_config struct {
	DELETE_STEP int
	REPEAT      int
}

type Delium_json_config struct {
	HASH    Delium_config
	MESSAGE Delium_config
}

type Dns_seed_config struct {
	UID     string
	Address string
}
