package mongo

import (
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ToObjectID ép hex 24 ký tự → bson.ObjectID. id sai định dạng → error rõ ràng
// (caller/gateway map thành 400 thay vì 500).
func ToObjectID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.NilObjectID, fmt.Errorf("id không hợp lệ: %s", id)
	}
	return oid, nil
}
