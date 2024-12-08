-- Enable UUID extension (if not already enabled)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Services table
CREATE TABLE services (
    service_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    host_address TEXT NOT NULL,
    port INT NOT NULL,
    email_id TEXT NOT NULL,
    password TEXT NOT NULL,
    cors_origin TEXT DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Templates table
CREATE TABLE templates (
    template_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster querying
CREATE INDEX idx_services_user_id ON services(user_id);
CREATE INDEX idx_templates_user_id ON templates(user_id);

-- Enable Row Level Security (RLS) on both tables
ALTER TABLE services ENABLE ROW LEVEL SECURITY;
ALTER TABLE templates ENABLE ROW LEVEL SECURITY;

-- Create policies for services table
CREATE POLICY "Users can view their own services"
    ON services FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own services"
    ON services FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update their own services"
    ON services FOR UPDATE
    USING (auth.uid() = user_id);

CREATE POLICY "Users can delete their own services"
    ON services FOR DELETE
    USING (auth.uid() = user_id);

-- Create policies for templates table
CREATE POLICY "Users can view their own templates"
    ON templates FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own templates"
    ON templates FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update their own templates"
    ON templates FOR UPDATE
    USING (auth.uid() = user_id);

CREATE POLICY "Users can delete their own templates"
    ON templates FOR DELETE
    USING (auth.uid() = user_id);

-- Create a function to automatically update the updated_at column
CREATE OR REPLACE FUNCTION update_modified_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to call the update_modified_column function
CREATE TRIGGER update_services_modtime
    BEFORE UPDATE ON services
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();

CREATE TRIGGER update_templates_modtime
    BEFORE UPDATE ON templates
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();

-- Add new fields to the templates table
ALTER TABLE templates
    ADD COLUMN to_email TEXT DEFAULT NULL,
    ADD COLUMN from_name TEXT NOT NULL,
    ADD COLUMN reply_to TEXT DEFAULT NULL,
    ADD COLUMN subject TEXT NOT NULL,
    ADD COLUMN bcc TEXT DEFAULT NULL,
    ADD COLUMN cc TEXT DEFAULT NULL;


-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Logs table
CREATE TABLE logs (
    log_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES templates(template_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Emails table
CREATE TABLE emails (
    email_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(service_id) ON DELETE CASCADE,
    template_id UUID NOT NULL REFERENCES templates(template_id) ON DELETE CASCADE,
    email_address TEXT NOT NULL,
    name TEXT DEFAULT NULL,
    phone_number TEXT DEFAULT NULL,
    sent_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);


-- Create indexes
CREATE INDEX idx_logs_user_id ON logs(user_id);
CREATE INDEX idx_logs_service_id ON logs(service_id);
CREATE INDEX idx_logs_template_id ON logs(template_id);
CREATE INDEX idx_emails_user_id ON emails(user_id);
CREATE INDEX idx_emails_service_id ON emails(service_id);
CREATE INDEX idx_emails_template_id ON emails(template_id);

-- Enable RLS
ALTER TABLE logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE emails ENABLE ROW LEVEL SECURITY;

-- RLS Policies for logs
CREATE POLICY "Users can view their own logs"
    ON logs FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own logs"
    ON logs FOR INSERT
    WITH CHECK (auth.uid() = user_id);

-- RLS Policies for emails
CREATE POLICY "Users can view their own sent emails"
    ON emails FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own sent emails"
    ON emails FOR INSERT
    WITH CHECK (auth.uid() = user_id);


-- Create the profile table
CREATE TABLE profile (
    profile_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- Unique ID for the profile
    user_id UUID UNIQUE NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE, -- One-to-one relationship with users table
    user_key TEXT UNIQUE NOT NULL, -- Unique key for the user
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- Timestamp for when the profile is created
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP  -- Timestamp for when the profile is updated
);

-- Enable Row Level Security (RLS) for the profile table
ALTER TABLE profile ENABLE ROW LEVEL SECURITY;

-- Create policies for the profile table
CREATE POLICY "Users can view their own profile"
    ON profile FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert their own profile"
    ON profile FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update their own profile"
    ON profile FOR UPDATE
    USING (auth.uid() = user_id);

CREATE POLICY "Users can delete their own profile"
    ON profile FOR DELETE
    USING (auth.uid() = user_id);

-- Create a trigger to update the updated_at column on profile updates
CREATE TRIGGER update_profile_modtime
    BEFORE UPDATE ON profile
    FOR EACH ROW
    EXECUTE FUNCTION update_modified_column();


ALTER TABLE profile
    ADD COLUMN rate_limit INT DEFAULT 300;

CREATE OR REPLACE FUNCTION reset_rate_limit()
RETURNS VOID AS $$
BEGIN
    UPDATE profile
    SET rate_limit = 300
    WHERE rate_limit < 300; -- Optional condition to reset only if rate_limit has been reduced
END;
$$ LANGUAGE plpgsql;



CREATE EXTENSION IF NOT EXISTS pg_cron;

SELECT cron.schedule('daily_rate_limit_reset', '0 0 * * *', $$CALL reset_rate_limit();$$);

