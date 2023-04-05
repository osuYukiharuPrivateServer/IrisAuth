package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	cookieNameForSessionID = "testtesttest"
	sess                   = sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
)

type Config struct {
	Appname string `json:"app_name"`
	Port    string `json:"app_port"`
	Mysql   string `json:"mysql_addr"`
}

type User struct {
	uid       int
	user_name string
	email     string
	passwd    string
	used_code string
	is_ban    bool
}

type CheckExist struct {
	exist bool
}

type CheckCode struct {
	exist        bool
	availability bool
}

type DoubleCheck struct {
	status bool
}

var conf Config

var db *sql.DB

func initDB() (err error) {
	// 不会校验账号密码是否正确
	// 注意！！！这里不要使用:=，我们是给全局变量赋值，然后在main函数中使用全局变量db
	db, err = sql.Open("mysql", conf.Mysql)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		fmt.Printf("connect to db failed, err:%v\n", err)
		return err
	}
	db.SetConnMaxLifetime(time.Second * 10)
	db.SetMaxOpenConns(200)
	db.SetMaxIdleConns(20)
	return err
}

func init() {
	file, _ := os.Open("config.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&conf)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println("Read Config Successfully")
	if err := initDB(); err != nil {
		fmt.Printf("init db failed,err:%v\n", err)
	}
	fmt.Println("Successfully connected to MySQL")
	fmt.Println("Iris will running @ ", conf.Port)
}

func main() {
	fmt.Println("APP Starts at: ", time.Now())

	app := iris.New()
	app.Logger().SetLevel("debug")
	app.RegisterView(iris.HTML("./frontend", ".html"))
	app.HandleDir("./static", http.Dir("./static"))

	app.Get("/", func(ctx iris.Context) {
		// Bind: {{.message}} with "Hello world!"
		ctx.ViewData("app_name", conf.Appname)
		ctx.ViewData("irisver", iris.Version)
		ctx.ViewData("mysqlver", "8.0.30")
		ctx.ViewData("gover", "18.3")
		// Render template file: ./views/hello.html
		ctx.View("index.html")
	})

	app.Get("/terms", func(ctx iris.Context) {
		ctx.ViewData("app_name", conf.Appname)
		ctx.ViewData("irisver", iris.Version)
		ctx.ViewData("mysqlver", "8.0.30")
		ctx.ViewData("gover", "18.3")
		ctx.View("terms.html")
	})

	app.Get("/login", loginget)
	app.Post("/login", loginpost)

	app.Get("/register", registerget)
	app.Post("/register", registerpost)

	app.Get("/doublecheck", doublecheck)

	app.OnErrorCode(iris.StatusNotFound, notFound)
	app.OnErrorCode(iris.StatusInternalServerError, internalServerError)

	app.Run(iris.Addr(":" + conf.Port))
}

func Flash(file string, status int, words string, ctx iris.Context) {
	ctx.ViewData("app_name", conf.Appname)
	if status == 1 { //error
		ctx.ViewData("error", true)
	}
	if status == 2 { //success
		ctx.ViewData("success", true)
	}
	if status == 3 { //ban
		ctx.ViewData("ban", true)
	}

	ctx.ViewData("flash", words)
	ctx.View(file + ".html")
}

func notFound(ctx iris.Context) {
	ctx.View("404.html")
}

func internalServerError(ctx iris.Context) {
	ctx.View("500.html")
}

func loginget(ctx iris.Context) {
	if ctx.URLParam("redirect_url") == "" || ctx.URLParam("check_url") == "" {
		ctx.Redirect("/")
	}
	ctx.ViewData("app_name", conf.Appname)
	session := sess.Start(ctx)
	session.Set("redirectURL", ctx.URLParam("redirect_url"))
	session.Set("checkURL", ctx.URLParam("check_url"))
	if session.GetString("LoginStatus") == "1" && session.GetString("uid") != "" { //先前已经成功登录过则直接跳转到double check页面，设置doublecheck为true
		sql2 := "UPDATE users SET doublecheck = ? WHERE uid = ?"
		row2 := db.QueryRow(sql2, true, session.GetString("uid"))
		err2 := row2.Scan()
		if err2 != nil {
			fmt.Println("doublecheck / Something Wrong Happens", err2)
		}
		ctx.Redirect(session.GetString("checkURL") + "/?redirect_url=" + session.GetString("redirectURL") + "?uid=" + session.GetString("uid"))
	}
	ctx.View("login.html")
}

func loginpost(ctx iris.Context) {
	session := sess.Start(ctx)
	email := ctx.PostValue("email")
	passwd := ctx.PostValue("password")
	if email == "" || passwd == "" {
		Flash("login", 3, "Oops! Fill in all the blanks first.", ctx)
	}
	status := checkpw(email, passwd)
	fmt.Println(status)
	if status == 1 { //success
		session.Set("LoginStatus", "1")
		sqlStr := "SELECT uid FROM users WHERE email = ?"
		var u User
		row := db.QueryRow(sqlStr, email)
		err := row.Scan(&u.uid)
		if err != nil {
			fmt.Println("LoginPost / Something Wrong Happens", err)
		}
		session.Set("uid", u.uid)
		ctx.Redirect(session.GetString("checkURL") + "/?redirect_url=" + session.GetString("redirectURL") + "&uid=" + strconv.Itoa(u.uid))
		fmt.Print("Login request: " + email + " " + passwd + " RedirectURL= " + session.GetString("redirectURL") + " CheckURL= " + session.GetString("checkURL") + ": Success.")
	}
	if status == 2 { //passwd incorrect
		Flash("login", 1, "Password Incorrect. Please try again.", ctx)
	}
	if status == 3 { //account banned
		Flash("login", 3, "Your Account has been banned. Please contact the support.", ctx)
	}
	if status == 4 { //account does not exist
		Flash("login", 3, "Account doesn't exist. Check your email address or register a new account.", ctx) //account does not exist
	}
}

func registerget(ctx iris.Context) {
	ctx.ViewData("app_name", conf.Appname)
	ctx.View("register.html")
}

func registerpost(ctx iris.Context) {
	ctx.ViewData("app_name", conf.Appname)
	username := ctx.PostValue("username")
	email := ctx.PostValue("email")
	passwd := ctx.PostValue("passwd")
	code := ctx.PostValue("code")
	if username == "" || email == "" || passwd == "" || code == "" {
		Flash("register", 3, "Oops! What are you doing?.", ctx)
	}
	status := register(username, email, passwd, code)
	if status == 0 { //Login Successfully
		Flash("login", 2, "Registered successfully. Here to login.", ctx)
	}
	if status == 1 { //Account already exist
		Flash("register", 1, "Account already exist. Try to login directly, or check your email.", ctx)
	}
	if status == 2 {
		Flash("register", 1, "邀请码不存在", ctx)
	}
	if status == 3 {
		Flash("register", 1, "邀请码不再可用", ctx)
	}
}

func checkpw(email, passwd string) int {
	if checkexist(email) == false {
		return 4
	}
	sqlStr := "SELECT uid, user_name, passwd, is_ban FROM users WHERE email = ?"
	var u User
	row := db.QueryRow(sqlStr, email)
	fmt.Println("row: ", row)
	err := row.Scan(&u.uid, &u.user_name, &u.passwd, &u.is_ban)
	if err != nil {
		fmt.Println("CheckPW / Something Wrong Happens", err)
		return 5
	}
	fmt.Println("err: ", err)
	if u.passwd != passwd {
		return 2
	}
	if u.is_ban == true {
		return 3
	}
	return 1
}

func register(username, email, passwd, code string) int {
	if checkexist(email) == true {
		return 1
	}
	checkCodeStat := checkCode(code)
	if checkCodeStat == 1 { //邀请码不存在
		return 2
	}
	if checkCodeStat == 2 { //邀请码不再可用
		return 3
	}
	sql := "INSERT INTO users (user_name, email, passwd, used_code) VALUES (?, ?, ?, ?)"
	row := db.QueryRow(sql, username, email, passwd, code)
	err := row.Scan()
	if err != nil {
		fmt.Println("register / Something Wrong Happens", err)
	}
	return 0
}

func doublecheck(ctx iris.Context) {
	if ctx.URLParam("uid") == "" {
		ctx.JSON(iris.Map{
			"err":     "1",
			"errhint": "URLParam 'uid' is needed for double check the login status",
		})
	}
	var stat DoubleCheck
	sql := "SELECT doublecheck FROM users WHERE uid = ?"
	row := db.QueryRow(sql, ctx.URLParam("uid"))
	err := row.Scan(&stat.status)
	if err != nil {
		fmt.Println("doublecheck / Something Wrong Happens", err)
	}
	if stat.status == true {
		sql2 := "UPDATE users SET doublecheck = ? WHERE uid = ?"
		row2 := db.QueryRow(sql2, false, ctx.URLParam("uid"))
		err2 := row2.Scan()
		if err2 != nil {
			fmt.Println("doublecheck / Something Wrong Happens", err)
		}
		var userinfo User
		sql3 := "SELECT uid, user_name, email FROM users WHERE uid = ?"
		row3 := db.QueryRow(sql3, ctx.URLParam("uid"))
		err := row3.Scan(&userinfo.uid, &userinfo.user_name, &userinfo.email)
		if err != nil {
			fmt.Println("doublecheck / Something Wrong Happens", err)
		}
		ctx.JSON(iris.Map{
			"status":    "0",
			"hint":      "login success",
			"uid":       userinfo.uid,
			"email":     userinfo.email,
			"user_name": userinfo.user_name,
		})
	} else {
		ctx.JSON(iris.Map{
			"status": "2",
			"hint":   "didn't login yet",
		})
	}
}

func checkexist(email string) bool {
	sql1 := "SELECT count(*) FROM users WHERE email = ?"
	var check CheckExist
	row2 := db.QueryRow(sql1, email)
	err := row2.Scan(&check.exist)
	if err != nil {
		fmt.Println("CheckExist / Something Wrong Happens", err)
	}
	fmt.Println(check.exist)
	if check.exist == true {
		return true
	} else {
		return false
	}
}

func checkCode(code string) int {
	sql2 := "SELECT count(*) FROM invite_code WHERE code = ?"
	var exist CheckExist
	row2 := db.QueryRow(sql2, code)
	err2 := row2.Scan(&exist.exist)
	if err2 != nil {
		fmt.Println("checkcode / Something Wrong Happens", err2)
	}
	if exist.exist == false {
		return 1
	}
	sql := "SELECT availability FROM invite_code WHERE code = ?"
	var check CheckCode
	row := db.QueryRow(sql, code)
	err := row.Scan(&check.availability)
	if err2 != nil {
		fmt.Println("checkcode / Something Wrong Happens", err)
	}
	if check.availability == false {
		return 2
	}
	return 0
}
