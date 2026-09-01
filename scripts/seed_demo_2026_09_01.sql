-- Datos de demostración para PostgreSQL.
-- Periodo: 2026-08-25 a 2026-09-08, tomando 2026-09-01 como fecha base.
-- No crea usuarios ADMIN.
-- OWNER/COLLABORATOR usan la contraseña: 12345678
-- WORKER inicia sesión mediante DNI: 91000001 a 91000050.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
DECLARE
    anchor_date CONSTANT date := DATE '2026-09-01';
    password_hash CONSTANT text := '$2a$10$SwZdWxJZE1QE/ZDpvNp1Y.MgcU84JVFZx.UazNXEXuYlZyXjp1Hla';
    owner_id uuid;
    rrhh_id uuid;
    collaborator_id uuid;
    v_worker_id uuid;
    v_assignment_id uuid;
    worker_number integer;
    collaborator_number integer;
    work_day date;
    selected_shift shift_type;
    selected_status meal_claim_status;
    selected_meal meal_type;
    service_day date;
    claim_time timestamptz;
    validation_time timestamptz;
    meal_number integer;
BEGIN
    -- OWNER: username seed_owner / password 12345678.
    SELECT user_id INTO owner_id
    FROM user_credentials
    WHERE type = 'PASSWORD' AND identifier = 'seed_owner';

    IF owner_id IS NULL THEN
        owner_id := gen_random_uuid();
        INSERT INTO users (id, role) VALUES (owner_id, 'OWNER');
        INSERT INTO user_profiles (user_id, first_name, last_name, email)
        VALUES (owner_id, 'Owner', 'Demostración', 'seed.owner@example.test');
        INSERT INTO user_credentials (user_id, type, identifier, secret_hash)
        VALUES (owner_id, 'PASSWORD', 'seed_owner', password_hash);
    ELSE
        UPDATE users SET role = 'OWNER', active = TRUE, updated_at = NOW() WHERE id = owner_id;
        UPDATE user_credentials SET secret_hash = password_hash, active = TRUE, updated_at = NOW()
        WHERE user_id = owner_id AND type = 'PASSWORD' AND identifier = 'seed_owner';
    END IF;

    -- RRHH: username seed_rrhh / password 12345678.
    SELECT user_id INTO rrhh_id
    FROM user_credentials
    WHERE type = 'PASSWORD' AND identifier = 'seed_rrhh';

    IF rrhh_id IS NULL THEN
        rrhh_id := gen_random_uuid();
        INSERT INTO users (id, role) VALUES (rrhh_id, 'RRHH');
        INSERT INTO user_profiles (user_id, first_name, last_name, email)
        VALUES (rrhh_id, 'Recursos Humanos', 'Demostración', 'seed.rrhh@example.test');
        INSERT INTO user_credentials (user_id, type, identifier, secret_hash)
        VALUES (rrhh_id, 'PASSWORD', 'seed_rrhh', password_hash);
    ELSE
        UPDATE users SET role = 'RRHH', active = TRUE, updated_at = NOW() WHERE id = rrhh_id;
        UPDATE user_credentials SET secret_hash = password_hash, active = TRUE, updated_at = NOW()
        WHERE user_id = rrhh_id AND type = 'PASSWORD' AND identifier = 'seed_rrhh';
    END IF;

    -- Cinco colaboradores: seed_collaborator01 ... seed_collaborator05.
    FOR collaborator_number IN 1..5 LOOP
        SELECT user_id INTO collaborator_id
        FROM user_credentials
        WHERE type = 'PASSWORD'
          AND identifier = 'seed_collaborator' || lpad(collaborator_number::text, 2, '0');

        IF collaborator_id IS NULL THEN
            collaborator_id := gen_random_uuid();
            INSERT INTO users (id, role) VALUES (collaborator_id, 'COLLABORATOR');
            INSERT INTO user_profiles (user_id, first_name, last_name, email)
            VALUES (
                collaborator_id,
                format('Colaborador %s', collaborator_number),
                'Demostración',
                'seed.collaborator' || lpad(collaborator_number::text, 2, '0') || '@example.test'
            );
            INSERT INTO user_credentials (user_id, type, identifier, secret_hash)
            VALUES (
                collaborator_id,
                'PASSWORD',
                'seed_collaborator' || lpad(collaborator_number::text, 2, '0'),
                password_hash
            );
        ELSE
            UPDATE users SET role = 'COLLABORATOR', active = TRUE, updated_at = NOW()
            WHERE id = collaborator_id;
            UPDATE user_credentials SET secret_hash = password_hash, active = TRUE, updated_at = NOW()
            WHERE user_id = collaborator_id
              AND type = 'PASSWORD'
              AND identifier = 'seed_collaborator' || lpad(collaborator_number::text, 2, '0');
        END IF;
    END LOOP;

    SELECT user_id INTO collaborator_id
    FROM user_credentials
    WHERE type = 'PASSWORD' AND identifier = 'seed_collaborator01';

    -- Cincuenta trabajadores con UUID y DNI propio.
    FOR worker_number IN 1..50 LOOP
        SELECT user_id INTO v_worker_id
        FROM user_credentials
        WHERE type = 'DNI'
          AND identifier = format('91%s', lpad(worker_number::text, 6, '0'));

        IF v_worker_id IS NULL THEN
            v_worker_id := gen_random_uuid();
            INSERT INTO users (id, role) VALUES (v_worker_id, 'WORKER');
            INSERT INTO user_profiles (user_id, first_name, last_name, email)
            VALUES (
                v_worker_id,
                format('Trabajador %s', lpad(worker_number::text, 2, '0')),
                'Demostración',
                format('seed.worker%s@example.test', lpad(worker_number::text, 2, '0'))
            );
            INSERT INTO user_credentials (user_id, type, identifier)
            VALUES (v_worker_id, 'DNI', format('91%s', lpad(worker_number::text, 6, '0')));
            INSERT INTO worker_information (
                user_id, employee_code, job_title, department, phone,
                hire_date, notes
            ) VALUES (
                v_worker_id,
                format('DEMO-%s', lpad(worker_number::text, 3, '0')),
                CASE WHEN worker_number % 2 = 0 THEN 'Operario' ELSE 'Auxiliar' END,
                CASE WHEN worker_number % 3 = 0 THEN 'Producción' ELSE 'Operaciones' END,
                '900' || lpad(worker_number::text, 6, '0'),
                anchor_date - ((worker_number % 365) + 30),
                'Generado por scripts/seed_demo_2026_09_01.sql'
            );
        ELSE
            UPDATE users SET role = 'WORKER', active = TRUE, updated_at = NOW() WHERE id = v_worker_id;
        END IF;

        -- Una semana anterior: CLOSED. Desde el 01/09 y una semana posterior: OPEN.
        FOR work_day IN
            SELECT generate_series(anchor_date - 7, anchor_date + 7, INTERVAL '1 day')::date
        LOOP
            selected_shift := CASE
                WHEN worker_number % 2 = 0 THEN 'DAY'::shift_type
                ELSE 'NIGHT'::shift_type
            END;

            INSERT INTO worker_shift_assignments (
                worker_id, shift_type, status, work_date, assigned_by, notes
            ) VALUES (
                v_worker_id,
                selected_shift,
                CASE WHEN work_day < anchor_date THEN 'CLOSED'::shift_status ELSE 'OPEN'::shift_status END,
                work_day,
                rrhh_id,
                'Turno de demostración'
            )
            ON CONFLICT ON CONSTRAINT worker_shift_assignments_one_shift_per_day DO UPDATE SET
                shift_type = EXCLUDED.shift_type,
                status = EXCLUDED.status,
                assigned_by = EXCLUDED.assigned_by,
                notes = EXCLUDED.notes,
                updated_at = NOW()
            RETURNING id INTO v_assignment_id;

            -- Solo las fechas históricas reciben meal_claims.
            IF work_day < anchor_date THEN
                FOR selected_meal, service_day, meal_number IN
                    SELECT meal, meal_date, ordinal
                    FROM (
                        SELECT 'BREAKFAST'::meal_type AS meal, work_day AS meal_date, 1 AS ordinal
                        WHERE selected_shift = 'DAY'
                        UNION ALL
                        SELECT 'LUNCH'::meal_type, work_day, 2
                        WHERE selected_shift = 'DAY'
                        UNION ALL
                        SELECT 'DINNER'::meal_type, work_day, 3
                        WHERE selected_shift = 'NIGHT'
                        UNION ALL
                        SELECT 'BREAKFAST'::meal_type, work_day + 1, 1
                        WHERE selected_shift = 'NIGHT'
                    ) meals
                LOOP
                    IF service_day >= anchor_date THEN
                        CONTINUE;
                    END IF;

                    selected_status := CASE (worker_number + EXTRACT(DAY FROM service_day)::integer + meal_number) % 3
                        WHEN 0 THEN 'VALIDATED'::meal_claim_status
                        WHEN 1 THEN 'NOT_CLAIMED'::meal_claim_status
                        ELSE 'CLAIMED_BUT_NOT_VALIDATED'::meal_claim_status
                    END;

                    claim_time := CASE selected_meal
                        WHEN 'BREAKFAST' THEN (service_day + TIME '07:15') AT TIME ZONE 'America/Lima'
                        WHEN 'LUNCH' THEN (service_day + TIME '13:15') AT TIME ZONE 'America/Lima'
                        ELSE (service_day + TIME '21:15') AT TIME ZONE 'America/Lima'
                    END;
                    validation_time := claim_time + INTERVAL '5 minutes';

                    INSERT INTO meal_claims (
                        worker_id, shift_assignment_id, meal_type, service_date,
                        claimed_at, status, validated_at, validated_by, notes,
                        created_at, updated_at
                    ) VALUES (
                        v_worker_id,
                        v_assignment_id,
                        selected_meal,
                        service_day,
                        CASE WHEN selected_status IN ('VALIDATED', 'CLAIMED_BUT_NOT_VALIDATED') THEN claim_time ELSE NULL END,
                        selected_status,
                        CASE WHEN selected_status = 'VALIDATED' THEN validation_time ELSE NULL END,
                        CASE WHEN selected_status = 'VALIDATED' THEN collaborator_id ELSE NULL END,
                        'Comida histórica de demostración',
                        service_day::timestamp AT TIME ZONE 'America/Lima',
                        CASE WHEN selected_status = 'VALIDATED' THEN validation_time ELSE service_day::timestamp AT TIME ZONE 'America/Lima' END
                    )
                    ON CONFLICT ON CONSTRAINT meal_claims_once_per_day DO UPDATE SET
                        shift_assignment_id = EXCLUDED.shift_assignment_id,
                        claimed_at = EXCLUDED.claimed_at,
                        status = EXCLUDED.status,
                        validated_at = EXCLUDED.validated_at,
                        validated_by = EXCLUDED.validated_by,
                        notes = EXCLUDED.notes,
                        updated_at = EXCLUDED.updated_at;
                END LOOP;
            END IF;
        END LOOP;
    END LOOP;
END $$;

COMMIT;

-- Resumen de verificación.
SELECT role, COUNT(*) AS quantity
FROM users
WHERE id IN (
    SELECT user_id FROM user_credentials
    WHERE identifier IN ('seed_owner', 'seed_rrhh')
       OR identifier LIKE 'seed_collaborator%'
       OR identifier BETWEEN '91000001' AND '91000050'
)
GROUP BY role
ORDER BY role;

SELECT status, COUNT(*) AS shifts
FROM worker_shift_assignments
WHERE notes = 'Turno de demostración'
GROUP BY status
ORDER BY status;

SELECT meal_type, status, COUNT(*) AS meals
FROM meal_claims
WHERE notes = 'Comida histórica de demostración'
GROUP BY meal_type, status
ORDER BY meal_type, status;
