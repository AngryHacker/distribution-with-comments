// Package silly provides a simple authentication scheme that checks for the
// existence of an Authorization header and issues access if is present and
// non-empty.
//
// This package is present as an example implementation of a minimal
// auth.AccessController and for testing. This is not suitable for any kind of
// production security.
package silly

import (
	"fmt"
	"net/http"
	"strings"

	ctxu "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/auth"
	"golang.org/x/net/context"
)

// accessController provides a simple implementation of auth.AccessController
// that simply checks for a non-empty Authorization header. It is useful for
// demonstration and testing.
type accessController struct {
	realm   string
	service string
}

var _ auth.AccessController = &accessController{}

// accessController 的创建函数
func newAccessController(options map[string]interface{}) (auth.AccessController, error) {
	// 检查 options 里的 realm 字段
	realm, present := options["realm"]
	if _, ok := realm.(string); !present || !ok {
		return nil, fmt.Errorf(`"realm" must be set for silly access controller`)
	}
	
	// 检查 options 里的 service 字段
	service, present := options["service"]
	if _, ok := service.(string); !present || !ok {
		return nil, fmt.Errorf(`"service" must be set for silly access controller`)
	}
	
	// 返回新的 accessController, 创建完毕
	return &accessController{realm: realm.(string), service: service.(string)}, nil
}

// Authorized simply checks for the existence of the authorization header,
// responding with a bearer challenge if it doesn't exist.
// 定义 accessController 的验证函数
func (ac *accessController) Authorized(ctx context.Context, accessRecords ...auth.Access) (context.Context, error) {
	// 取得 http 请求头部
	req, err := ctxu.GetRequest(ctx)
	if err != nil {
		return nil, err
	}

    // 检查头部里的 Authorization 字段
	if req.Header.Get("Authorization") == "" {
		challenge := challenge{
			realm:   ac.realm,
			service: ac.service,
		}

		if len(accessRecords) > 0 {
			var scopes []string
			// 把动作写入 scopes 
			for _, access := range accessRecords {
				scopes = append(scopes, fmt.Sprintf("%s:%s:%s", access.Type, access.Resource.Name, access.Action))
			}
			// join 之后返回
			challenge.scope = strings.Join(scopes, " ")
		}
		
		// 返回 challenge 信息
		return nil, &challenge
	}
	
	// 返回经过 auth 用户 silly
	return auth.WithUser(ctx, auth.UserInfo{Name: "silly"}), nil
}

type challenge struct {
	realm   string
	service string
	scope   string
}

// challenge 方法将未认证的信息格式化为 HTTP 格式并输出
func (ch *challenge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := fmt.Sprintf("Bearer realm=%q,service=%q", ch.realm, ch.service)

	if ch.scope != "" {
		header = fmt.Sprintf("%s,scope=%q", header, ch.scope)
	}

	w.Header().Set("WWW-Authenticate", header)
	w.WriteHeader(http.StatusUnauthorized)
}

// challenge 的 Error 方法
func (ch *challenge) Error() string {
	return fmt.Sprintf("silly authentication challenge: %#v", ch)
}

// init registers the silly auth backend.
func init() {
	// 注册 silly 情景构造函数
	auth.Register("silly", auth.InitFunc(newAccessController))
}
