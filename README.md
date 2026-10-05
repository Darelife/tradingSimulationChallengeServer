# Registering

```bash
curl -X POST http://<url>/register/start \
  -H "Content-Type: application/json" \
  -d '{"email":"<yourBitsMail>"}'
  
curl -X POST http://<url>/register/complete \
  -H "Content-Type: application/json" \
  -d '{"challenge_id":"<challenge_id>","code":"<code>","password":"<password>"}'
```
