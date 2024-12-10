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

	findOptions := options.Find().SetSort(config.DESC).SetLimit(utils.StringToInt64(limit)).SetSkip(utils.StringToInt64(skip)).SetProjection(config.SET_ID_TO_ZERO)
	cursor, find_err := config.Data_COLL.Find(context.TODO(), filter, findOptions)
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
	var finded_Data Data
	finded_Data_err := FindOne(config.Data_COLL, bson.M{"Datatype": config.GENESIS_DATA_TYPE}, &finded_Data)

	if finded_Data_err == mongo.ErrNoDocuments {

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

	// Check for DNS seed
	cursor, find_err := config.DNS_SEED_COLL.Find(context.TODO(), config.NO_FILTER)
	if find_err != nil {
		return false, config.ErrFindMany
	}
	cursor_err := cursor.All(context.TODO(), &DNS_SEED_LIST)
	if cursor_err != nil {
		return false, config.ErrCursor
	}

	if len(DNS_SEED_LIST) == 0 {

		DNS_SEED_LIST := []interface{}{
			DnsSeed{
				UID: config.GENESIS_DNS_SEED_1,
			},
		}

		_, insert_many_dns_err := config.DNS_SEED_COLL.InsertMany(context.TODO(), DNS_SEED_LIST)

		if insert_many_dns_err != nil {
			return false, insert_many_dns_err
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

	signature, _ := utils.Sign(config.NODE_PRIVKEY, fullData)
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
	t.Hash = utils.D256(message, config.HASH_DELIUM_CONFIG.Delete_step, config.HASH_DELIUM_CONFIG.Repeat).String
}

func DataFilterMaker(uid string) bson.M {
	return bson.M{
		"$or": []bson.M{
			{"senderuid": uid},
			{"receiveruid": uid},
		},
	}
}
