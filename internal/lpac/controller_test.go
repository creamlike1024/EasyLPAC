package lpac

//func TestController_HandleOutput(t *testing.T) {
//	controller := LoadFixture("chip-info")
//	stdout := bytes.NewReader([]byte(`
//{"type":"progress","payload":{"code":0,"message":"es10b_get_euicc_challenge_and_info","data":null}}
//{"type":"progress","payload":{"code":0,"message":"es9p_initiate_authentication","data":null}}
//{"type":"progress","payload":{"code":0,"message":"es10b_authenticate_server","data":null}}
//{"type":"progress","payload":{"code":0,"message":"es9p_authenticate_client","data":null}}
//{"type":"lpa","payload":{"code":-1,"message":"es9p_authenticate_client","data":"EID doesn’t match the expected value, or there is no pending RSP session for this eUICC."}}
//`))
//	_, err := controller.handle(nil, nil, nil, stdout)
//	assert.Error(t, err, "es9p_authenticate_client: EID doesn’t match the expected value, or there is no pending RSP session for this eUICC.")
//}
