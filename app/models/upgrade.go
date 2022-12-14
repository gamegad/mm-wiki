package models

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/chaiyd/mm-wiki/app/utils"
	"github.com/chaiyd/mm-wiki/global"
	"time"
)

type Upgrade struct {
}

type upgradeHandleFunc func() error

type upgradeHandle struct {
	Version string
	Func    upgradeHandleFunc
}

var (
	UpgradeModel = Upgrade{}

	upgradeMap = []*upgradeHandle{}
)

// upgrade handle func
func (up *Upgrade) initHandleFunc() {
	// v0 ~ v0.1.2
	upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.1.2", Func: up.v0ToV012})

	// v0.1.2 ~ v0.1.3
	upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.1.3", Func: up.v012ToV013})

	// v0.1.3 ~ v0.1.8
	upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.1.8", Func: up.v013ToV018})

	// v0.1.8 ~ v0.2.0
	upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.2.0", Func: up.v018ToV020})

	// v0.2.1 ~ v0.2.7
	//upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.2.7", Func: up.v021ToV027})
	// v0.2.7 ~ v0.3.3
	//upgradeMap = append(upgradeMap, &upgradeHandle{Version: "v0.3.3", Func: up.v027ToV033})
}

// upgrade start
func (up *Upgrade) Start(dbVersion string) (err error) {
	up.initHandleFunc()

	var tmpVersion = dbVersion
	for _, upHandle := range upgradeMap {
		// upgrade now version, exit
		if tmpVersion == global.SYSTEM_VERSION {
			break
		}
		// tmpVersion < upHandle.version
		if utils.VersionCompare.Lt(tmpVersion, upHandle.Version) {
			// upgrade handle
			err = upHandle.Func()
			if err != nil {
				logs.Error("upgrade to " + upHandle.Version + " error: " + err.Error())
				return errors.New("upgrade to " + upHandle.Version + " error: " + err.Error())
			}
			// update system database version
			err = up.upgradeAfter(upHandle.Version)
			if err != nil {
				logs.Error("upgrade to database " + upHandle.Version + " error: " + err.Error())
				return errors.New("upgrade to database " + upHandle.Version + " error: " + err.Error())
			}
			logs.Info("upgrade to " + upHandle.Version + " success")
			// update version record
			tmpVersion = upHandle.Version
		}
	}
	// last update current version
	err = up.upgradeAfter(global.SYSTEM_VERSION)
	if err != nil {
		logs.Error("upgrade to database " + global.SYSTEM_VERSION + " error: " + err.Error())
		return errors.New("upgrade to database " + global.SYSTEM_VERSION + " error: " + err.Error())
	}
	logs.Info("upgrade finish, version: " + global.SYSTEM_VERSION)

	return nil
}

// upgrade v0.0.0 ~ v0.1.2
func (up *Upgrade) v0ToV012() (err error) {

	// 1. add privilege '/email/test'
	// INSERT INTO mw_privilege (name, parent_id, type, controller, action, icon, target, is_display, sequence, create_time, update_time) VALUES ('?????????????????????', 53, 'controller', 'email', 'test', 'glyphicon-list', 0, 80, unix_timestamp(now()), unix_timestamp(now()));
	privilege := map[string]interface{}{
		"name":       "?????????????????????",
		"type":       "controller",
		"parent_id":  53,
		"controller": "email",
		"action":     "test",
		"target":     "",
		"icon":       "glyphicon-list",
		"is_display": 0,
		"sequence":   80,
	}
	_, err = PrivilegeModel.InsertNotExists(privilege)
	if err != nil {
		return
	}

	// 2. table mw_email add field 'is_ssl'
	// alter table mw_email add `is_ssl` tinyint(1) NOT NULL DEFAULT '0' COMMENT '????????????ssl??? 0 ??????????????? 1 ??????' after `password`
	db := G.DB()
	db.Exec(db.AR().Raw("alter table mw_email DROP COLUMN `is_ssl`"))
	_, err = db.Exec(db.AR().Raw("alter table mw_email add `is_ssl` tinyint(1) NOT NULL DEFAULT '0' COMMENT '????????????ssl??? 0 ??????????????? 1 ??????' after `password`"))

	return
}

// upgrade v0.1.2 ~ v0.1.3
func (up *Upgrade) v012ToV013() error {

	// create attachment table
	sql := "DROP TABLE IF EXISTS `mw_attachment`;" +
		"CREATE TABLE `mw_attachment` (" +
		"`attachment_id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '?????? id'," +
		"`user_id` int(10) NOT NULL DEFAULT '0' COMMENT '????????????id'," +
		"`document_id` int(10) NOT NULL DEFAULT '0' COMMENT '????????????id'," +
		"`name` varchar(50) NOT NULL DEFAULT '' COMMENT '????????????'," +
		"`path` varchar(100) NOT NULL DEFAULT '' COMMENT '????????????'," +
		"`source` tinyint(1) NOT NULL DEFAULT '0' COMMENT '??????????????? 0 ??????????????? 1 ??????'," +
		"`create_time` int(11) NOT NULL DEFAULT '0' COMMENT '????????????'," +
		"`update_time` int(11) NOT NULL DEFAULT '0' COMMENT '????????????'," +
		"PRIMARY KEY (`attachment_id`)," +
		"KEY (`document_id`, `source`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='???????????????';"

	return up.createTable(sql)
}

// upgrade v0.1.3 ~ v0.1.8
func (up *Upgrade) v013ToV018() error {
	db := G.DB()

	// 1. ????????????????????? space_id ??????
	// `space_id` int(10) NOT NULL DEFAULT '0' COMMENT '??????id'
	_, err := db.Exec(db.AR().Raw("alter table mw_log_document add `space_id` int(10) NOT NULL DEFAULT '0' COMMENT '??????ID'"))
	if err == nil {
		// ????????????????????? space_id
		_, err = db.Exec(db.AR().Raw("update mw_log_document as logDocment, mw_document as document set logDocment.space_id = document.space_id WHERE logDocment.document_id = document.document_id"))
		if err != nil {
			return err
		}
	}

	// 2. ??????????????????????????????????????????????????????????????????????????????????????????sequence=0???
	_, err = db.Exec(db.AR().Raw("update mw_document set mw_document.sequence = mw_document.document_id WHERE sequence=0"))
	if err != nil {
		return err
	}

	return nil
}

// upgrade v0.1.8 ~ v0.2.0
func (up *Upgrade) v018ToV020() error {
	// ?????????????????????
	db := G.DB()
	updateTime := time.Now().Unix()
	// 1. ?????????????????????????????????
	insertSql := fmt.Sprintf("INSERT INTO `mw_config` (name, `key`, value, create_time, update_time) VALUES ('??????????????????', 'fulltext_search_open', '1', %d, %d)", updateTime, updateTime)
	_, err := db.Exec(db.AR().Raw(insertSql))
	if err != nil {
		return err
	}
	// 2. ???????????????????????????????????????
	insertSql = fmt.Sprintf("INSERT INTO `mw_config` (name, `key`, value, create_time, update_time) VALUES ('??????????????????', 'doc_search_timer', '3600', %d, %d)", updateTime, updateTime)
	_, err = db.Exec(db.AR().Raw(insertSql))
	if err != nil {
		return err
	}
	// 3. ?????????????????????????????????
	insertSql = fmt.Sprintf("INSERT INTO `mw_config` (name, `key`, value, create_time, update_time) VALUES ('????????????', 'system_name', 'Markdown Mini Wiki', %d, %d)", updateTime, updateTime)
	_, err = db.Exec(db.AR().Raw(insertSql))
	if err != nil {
		return err
	}
	// 4. ????????????????????????????????????
	// INSERT INTO mw_privilege (privilege_id, name, parent_id, type, controller, action, icon, target, is_display, sequence, create_time, update_time) VALUES (93, '???????????????', 71, 'controller', 'contact', 'import', 'glyphicon-list', '', 0, 97, unix_timestamp(now()), unix_timestamp(now()));
	privilege := map[string]interface{}{
		"name":       "???????????????",
		"type":       "controller",
		"parent_id":  71,
		"controller": "contact",
		"action":     "import",
		"target":     "",
		"icon":       "glyphicon-list",
		"is_display": 0,
		"sequence":   97,
	}
	_, err = PrivilegeModel.InsertNotExists(privilege)
	if err != nil {
		return err
	}
	return nil
}

// upgrade v0.2.1 ~ v0.2.7
func (up *Upgrade) v021ToV027() error {
	return nil
}

// upgrade v0.2.7 ~ v0.3.3
func (up *Upgrade) v027ToV033() error {
	return nil
}

func (up *Upgrade) createTable(sqlTable string) error {
	host := beego.AppConfig.String("db::host")
	port, _ := beego.AppConfig.Int("db::port")
	user := beego.AppConfig.String("db::user")
	pass := beego.AppConfig.String("db::pass")
	name := beego.AppConfig.String("db::name")
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&multiStatements=true", user, pass, host, port, name))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(sqlTable)
	if err != nil {
		return err
	}
	return nil
}

func (up *Upgrade) upgradeAfter(version string) (err error) {
	// update system version
	config, err := ConfigModel.GetConfigByKey(ConfigKeySystemVersion)
	if err != nil {
		return
	}
	if len(config) == 0 {
		configValue := map[string]interface{}{
			"name":  "???????????????",
			"key":   "system_version",
			"value": version,
		}
		_, err = ConfigModel.Insert(configValue)
	} else {
		_, err = ConfigModel.UpdateByKey(ConfigKeySystemVersion, version)
	}

	return err
}
