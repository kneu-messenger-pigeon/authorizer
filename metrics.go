package main

import "github.com/VictoriaMetrics/metrics"

var getAuthUrlRequestsTotal = metrics.NewCounter(`get_auth_url_requests_total`)
var completeAuthRequestsTotal = metrics.NewCounter(`complete_auth_requests_total`)

var authErrorExpiredStateTotal = metrics.NewCounter(`auth_error{error="expired_state"}`)
var authErrorWrongStateTotal = metrics.NewCounter(`auth_error{error="wrong_state"}`)
var authErrorFailGetTokenTotal = metrics.NewCounter(`auth_error{error="fail_get_token"}`)
var authErrorFailGetUserTotal = metrics.NewCounter(`auth_error{error="fail_get_user"}`)
var authErrorFailFinishAuthTotal = metrics.NewCounter(`auth_error{error="fail_finish_auth"}`)
