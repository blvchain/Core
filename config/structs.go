package config

type Delium_config struct {
	DELETE_STEP int
	REPEAT      int
}

type Delium_json_config struct {
	HASH    Delium_config
	MESSAGE Delium_config
}

type Blv_info_json struct {
	PASSWORD    string
	PRIVATE_KEY string
	PUBLIC_KEY  string
	UID         string
}
