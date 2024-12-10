package db

import (
	"context"

	"blvchain/core/config"
	"blvchain/core/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func FindManyDatas(filter primitive.M, skip string, limit string) ([]Data, error) {
	var result []Data

	if skip == "" {
		skip = "0"
	}
	if limit == "" {
		limit = "1"
	}

	findOptions := options.Find().SetSort(config.DESC).SetLimit(utils.StringToInt64(limit)).SetSkip(utils.StringToInt64(skip))
	cursor, find_err := config.DATA_COLL.Find(context.TODO(), filter, findOptions)
	if find_err != nil {
		return result, config.ErrFindMany
	}
	cursor_err := cursor.All(context.TODO(), &result)
	if cursor_err != nil {
		return result, config.ErrCursor
	}

	if result == nil {
		return result, mongo.ErrNoDocuments
	}

	return result, nil
}

func Genesis_check() (bool, error) {

	// Check for first Data
	var genesis_data Data
	genesis_data_err := FindOne(config.DATA_COLL, bson.M{"predatahash": config.GENE}, &genesis_data)

	if genesis_data_err == mongo.ErrNoDocuments {

		genesis_Data := Data{
			PreDataHash: config.GENESIS_DATA_PREHASH,
			SenderUID:   config.NODE_URL,
			ReceiverUID: config.MAKER_UID,
			TimeStamp:   config.GENESIS_TIMESTAMP,
			NodeData: NodeData{
				NodeID:    config.BLVCHAIN_URL,
				PubKey:    config.BLVCHAIN_PUBKEY,
				Signature: config.GENESIS_SIGNATURE,
			},
		}

		DataHashMaker(&genesis_Data)

		Data_insert_result, Data_insert_result_err := InsertOne(config.Data_COLL, genesis_Data, "hash")
		if !Data_insert_result {
			return false, Data_insert_result_err
		}

	}

	return true, nil
}

func NodeSignatureMaker(t *Data) {
	fullData := t.PreDataHash + config.DELIMITER +
		t.Hash + config.DELIMITER +
		t.SenderUID + config.DELIMITER +
		t.SenderPubKey + config.DELIMITER +
		t.Signature + config.DELIMITER +
		t.ReceiverUID + config.DELIMITER +
		utils.Int64ToStr(t.TimeStamp)

	signature, _ := utils.Sign(config.BLV_INFO.PRIVATE_KEY, fullData)
	t.NodeData.Signature = signature
}

func Message_maker(t Data) string {
	return t.PreDataHash + config.DELIMITER +
		t.SenderUID + config.DELIMITER +
		t.ReceiverUID + config.DELIMITER +
		utils.Int64ToStr(t.TimeStamp) + config.DELIMITER +
		t.NodeData.NodeID
}

func DataHashMaker(t *Data) {
	message := Message_maker(*t)
	t.Hash = utils.D256(message, config.DELIUM_CONFIG.HASH.DELETE_STEP, config.DELIUM_CONFIG.HASH.REPEAT).String
}

func DataFilterMaker(uid string) bson.M {
	return bson.M{
		"$or": []bson.M{
			{"senderuid": uid},
			{"receiveruid": uid},
		},
	}
}
