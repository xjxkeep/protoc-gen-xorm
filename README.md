# protoc-gen-xorm

protoc 插件，把 protobuf 转换成 xorm 需要用到的钩子代码。

## 安装

```bash
go get git@g.echo.tech:infra-mysql/protoc-gen-xorm.git
```

## 使用

在生成 proto 到 golang 代码时，加上：`--xorm_out=`

```bash
protoc \
    -I $GRPC_PROTO_SRC_DIR \
    --go_out=$GRPC_PROTO_DST_DIR \
    --go-json_out=allow_unknown,emit_defaults:$GRPC_PROTO_DST_DIR \
    --micro_out=micro,service_name=$GRPC_NAME:$GRPC_PROTO_DST_DIR \
    --xorm_out=$GRPC_PROTO_DST_DIR
```

然后会生成类似下面的 golang 代码，`FromDB` 和 `ToDB` 是 xorm 的 [hook 方法](https://xorm.io/docs/chapter-02/4.columns/)：

```golang
// FromDB implements xorm.Conversion.FromDB
func (x *ExpressOrderStatus) FromDB(bytes []byte) error {
	values := ExpressOrder_Status_value
	key := string(bytes)

	value := int32(0)
	if v, ok := values[key]; ok {
		value = v
	} else if v, ok := values["STATUS"+"_"+key]; ok {
		value = v
	}

	*x = ExpressOrder_Status(value)
	return nil
}

// ToDB implements xorm.Conversion.ToDB
func (x *ExpressOrderStatus) ToDB() ([]byte, error) {
	name := ExpressOrder_Status_name[int32(*x)]
	return []byte(strings.TrimPrefix(name, "STATUS"+"_")), nil
}

// Value when parser where args
func (x *ExpressOrderStatus) Value() (driver.Value, error){
	return ExpressOrder_Status_name[int32(x)], nil
}
```

## 举个例子

发货单有个状态的枚举字段，值有待发货，待收货，取消，待配货。

> 前提：在生成 golang 代码时需要带上此插件，比如 https://g.echo.tech/dev/express/-/blob/master/build.sh#L23

MySQL DDL 语句：

```sql
CREATE TABLE `express_orders` (
  `id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '主键',
  `status` enum('DEFAULT', 'WAIT_SEND','WAIT_RECEIVE','CANCELED','WAIT_PRE_ALLOCATE') NOT NULL COMMENT '状态',
)
```

💡 **protobuf 定义的枚举顺序必须和数据库的一致**：

```protobuf
 // 顺序请与数据库表设计保持一致
enum ExpressOrderStatus {
    STATUS_PLACEHOLDER = 0;
    STATUS_DEFAULT = 1;
    STATUS_WAIT_SEND = 2; // 等待发货
    STATUS_WAIT_RECEIVE = 3; // 等待收货
    STATUS_CANCELED = 4; // 取消
    STATUS_WAIT_PRE_ALLOCATE = 5; // 等待预分配货物
}
```

那么定义 model 结构体时就可以引用上面 pb 定义的枚举：

```golang
// model
type ExpressOrder struct {
	ID     uint64
	Status pb.ExpressOrderStatus
}

// 插入的时候，xorm 会调用 ExpressOrder 每个字段类型的钩子方法，比如 ToDB
func InsertExpressOrder(order *ExpressOrder) error {
    return connection.Insert(order)
}
```

