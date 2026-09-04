INSERT INTO user_credentials (user_id, type, identifier, secret_hash)
SELECT dni.user_id, 'PASSWORD', dni.user_id::text, crypt(dni.identifier, gen_salt('bf'))
FROM user_credentials dni
JOIN users u ON u.id = dni.user_id AND u.role = 'WORKER'
WHERE dni.type = 'DNI'
  AND dni.active = TRUE
  AND NOT EXISTS (
      SELECT 1
      FROM user_credentials password
      WHERE password.user_id = dni.user_id
        AND password.type = 'PASSWORD'
  );
