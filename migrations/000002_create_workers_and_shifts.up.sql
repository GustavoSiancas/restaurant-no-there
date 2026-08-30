CREATE TYPE shift_type AS ENUM ('DIA', 'NOCHE');

-- Información laboral exclusiva de usuarios con rol WORKER.
-- user_id es la clave primaria para garantizar la relación uno a uno.
CREATE TABLE worker_information (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    employee_code VARCHAR(50) NOT NULL UNIQUE,
    job_title VARCHAR(120),
    department VARCHAR(120),
    phone VARCHAR(30),
    address TEXT,
    hire_date DATE,
    emergency_contact_name VARCHAR(200),
    emergency_contact_phone VARCHAR(30),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT worker_information_employee_code_not_blank
        CHECK (BTRIM(employee_code) <> '')
);

-- Catálogo de turnos reutilizables. Un turno define sus horas y explica
-- mediante description cuál es su propósito operativo.
CREATE TABLE shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    type shift_type NOT NULL,
    description TEXT NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT shifts_name_not_blank CHECK (BTRIM(name) <> ''),
    CONSTRAINT shifts_description_not_blank CHECK (BTRIM(description) <> ''),
    CONSTRAINT shifts_different_times CHECK (start_time <> end_time)
);

-- Calendario rotativo: cada fila representa el turno de un trabajador
-- en una fecha. assigned_by registra quién organizó la asignación.
CREATE TABLE worker_shift_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    worker_id UUID NOT NULL REFERENCES worker_information(user_id) ON DELETE CASCADE,
    shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE RESTRICT,
    work_date DATE NOT NULL,
    assigned_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT worker_shift_assignments_one_shift_per_day
        UNIQUE (worker_id, work_date)
);

CREATE INDEX worker_shift_assignments_date_idx
    ON worker_shift_assignments (work_date);

CREATE INDEX worker_shift_assignments_shift_date_idx
    ON worker_shift_assignments (shift_id, work_date);

-- Datos iniciales editables. El turno nocturno cruza medianoche cuando
-- end_time es menor que start_time.
INSERT INTO shifts (name, type, description, start_time, end_time)
VALUES
    ('Turno diurno', 'DIA',
     'Jornada realizada durante el día. La hora de salida ocurre en la misma fecha de la asignación.',
     '08:00', '17:00'),
    ('Turno nocturno', 'NOCHE',
     'Jornada que comienza en la fecha asignada y finaliza al día siguiente cuando la hora de salida es menor que la hora de entrada.',
     '20:00', '05:00');

