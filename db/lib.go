package db

import (
	"blvchain/core/config"
	"blvchain/core/utils"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

func Genesis_check() (bool, error) {

	// Check for first Data
	var genesis_data Data = Data{
		PreDataHash:  config.GENESIS_DATA_PREHASH,
		SenderUID:    config.GENESIS_SENDER_UID,
		SenderIndex:  1,
		SenderPubKey: config.GENESIS_PUBKEY,
		Signature:    config.GENESIS_SIGNATURE,
		Data:         config.GENESIS_DATA,
		SenderRole:   config.GENESIS_SENDER_ROLE,
		TimeStamp:    config.GENESIS_TIMESTAMP,
		NodeData: NodeData{
			NodeUID:   config.GENESIS_NODE_UID,
			Signature: config.GENESIS_NODE_SIGNATURE,
		},
	}
	DataHashMaker(&genesis_data)

	db_genesis_datas, _ := FindAllDatas(bson.M{"predatahash": config.GENESIS_DATA_PREHASH})

	// No genesis data
	if len(db_genesis_datas) == 0 {

		Data_insert_result, Data_insert_result_err := InsertOne(config.DATA_COLL, genesis_data, "hash")
		if !Data_insert_result {
			return false, Data_insert_result_err
		}

	} else {

		var count_genesis_data = 0

		// Make sure just have pure genesis data
		for _, db_data := range db_genesis_datas {
			if !reflect.DeepEqual(db_data, genesis_data) {
				config.PrintError("error: Found NOT genesis data with genesis prehash in DB")
				count_genesis_data += 1
			}
		}

		if count_genesis_data > 1 {
			config.PrintError("error: Found more than one genesis data")
		}
	}

	return true, nil
}

func NodeSignatureMaker(t *Data) {
	fullData := t.PreDataHash + config.DELIMITER +
		t.Hash + config.DELIMITER +
		t.SenderUID + config.DELIMITER +
		utils.Int64ToStr(t.SenderIndex) + config.DELIMITER +
		t.SenderPubKey + config.DELIMITER +
		t.Signature + config.DELIMITER +
		t.ReceiverUID + config.DELIMITER +
		t.Data + config.DELIMITER +
		t.DataHash + config.DELIMITER +
		utils.Int64ToStr(t.SenderRole) + config.DELIMITER +
		utils.Int64ToStr(t.ReceiverRole) + config.DELIMITER +
		utils.Int64ToStr(t.TimeStamp)

	signature, _ := utils.Sign(config.BLV_INFO.PRIVATE_KEY, fullData)

	t.NodeData = NodeData{
		NodeUID:   config.BLV_INFO.UID,
		Signature: signature,
	}
}

func Message_maker(t Data) string {
	return t.SenderUID + config.DELIMITER +
		t.ReceiverUID + config.DELIMITER +
		t.Data + config.DELIMITER +
		t.DataHash + config.DELIMITER +
		utils.Int64ToStr(t.TimeStamp) + config.DELIMITER
}

func DataHashMaker(t *Data) {
	message := Message_maker(*t)
	t.MessageHash = message
	t.Hash = utils.D256(message, config.DELIUM_CONFIG.HASH.DELETE_STEP, config.DELIUM_CONFIG.HASH.REPEAT).String
}
