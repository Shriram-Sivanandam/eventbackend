CREATE TABLE page_members (
  page_id UUID REFERENCES pages(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,

  role TEXT CHECK (role IN ('owner', 'admin', 'member')) NOT NULL,

  joined_at TIMESTAMPTZ DEFAULT NOW(),

  PRIMARY KEY (page_id, user_id)
);
