DELETE FROM user_credentials password
USING users u
WHERE password.user_id = u.id
  AND u.role = 'WORKER'
  AND password.type = 'PASSWORD'
  AND password.identifier = password.user_id::text;
