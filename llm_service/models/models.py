from sqlalchemy import Column, Integer, String, DateTime, ForeignKey, Text, Enum, Float, Date
from sqlalchemy.orm import relationship
from sqlalchemy.ext.declarative import declarative_base
from datetime import datetime
import enum

Base = declarative_base()

class Project(Base):
    __tablename__ = 'projects'

    id = Column(Integer, primary_key=True)
    name = Column(String(255), nullable=False)
    description = Column(Text)
    start_date = Column(Date)
    end_date = Column(Date)
    status = Column(String(50), default="active")
    budget = Column(Float)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    tasks = relationship("Task", back_populates="project")
    requirements = relationship("Requirement", back_populates="project")
    team_members = relationship("TeamMember", back_populates="project")

class TaskStatus(enum.Enum):
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"
    ON_HOLD = "on_hold"
    REVIEW = "review"
    BLOCKED = "blocked"

class Task(Base):
    __tablename__ = 'tasks'

    id = Column(Integer, primary_key=True)
    project_id = Column(Integer, ForeignKey('projects.id'))
    title = Column(String(255), nullable=False)
    description = Column(Text)
    status = Column(Enum(TaskStatus), default=TaskStatus.PENDING)
    priority = Column(Integer, default=1)
    due_date = Column(Date)
    estimated_hours = Column(Float)
    actual_hours = Column(Float)
    assignee_id = Column(Integer, ForeignKey('team_members.id'))
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    project = relationship("Project", back_populates="tasks")
    assignee = relationship("TeamMember", back_populates="assigned_tasks")
    code_snippets = relationship("CodeSnippet", back_populates="task")
    execution_results = relationship("ExecutionResult", back_populates="task")
    web_searches = relationship("WebSearch", back_populates="task")

class CodeSnippet(Base):
    __tablename__ = 'code_snippets'

    id = Column(Integer, primary_key=True)
    task_id = Column(Integer, ForeignKey('tasks.id'))
    content = Column(Text, nullable=False)
    language = Column(String(50))
    version = Column(Integer, default=1)
    author_id = Column(Integer, ForeignKey('team_members.id'))
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    task = relationship("Task", back_populates="code_snippets")
    author = relationship("TeamMember", back_populates="authored_snippets")

class RequirementStatus(enum.Enum):
    DRAFT = "draft"
    APPROVED = "approved"
    IMPLEMENTED = "implemented"
    REJECTED = "rejected"
    UNDER_REVIEW = "under_review"

class RequirementType(enum.Enum):
    FUNCTIONAL = "functional"
    NON_FUNCTIONAL = "non_functional"
    TECHNICAL = "technical"
    BUSINESS = "business"

class Requirement(Base):
    __tablename__ = 'requirements'

    id = Column(Integer, primary_key=True)
    project_id = Column(Integer, ForeignKey('projects.id'))
    content = Column(Text, nullable=False)
    priority = Column(Integer, default=1)
    status = Column(Enum(RequirementStatus), default=RequirementStatus.DRAFT)
    type = Column(Enum(RequirementType))
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    project = relationship("Project", back_populates="requirements")

class ExecutionResult(Base):
    __tablename__ = 'execution_results'

    id = Column(Integer, primary_key=True)
    task_id = Column(Integer, ForeignKey('tasks.id'))
    output = Column(Text)
    error = Column(Text)
    execution_time = Column(Float)
    status = Column(String(50), default="success")
    environment = Column(String(100))
    parameters = Column(Text)
    created_at = Column(DateTime, default=datetime.utcnow)

    task = relationship("Task", back_populates="execution_results")

class WebSearch(Base):
    __tablename__ = 'web_searches'

    id = Column(Integer, primary_key=True)
    task_id = Column(Integer, ForeignKey('tasks.id'))
    query = Column(String(255), nullable=False)
    result = Column(Text)
    search_engine = Column(String(50), default="google")
    category = Column(String(50))
    filters = Column(Text)
    timestamp = Column(DateTime, default=datetime.utcnow)

    task = relationship("Task", back_populates="web_searches")

class TeamMember(Base):
    __tablename__ = 'team_members'

    id = Column(Integer, primary_key=True)
    project_id = Column(Integer, ForeignKey('projects.id'))
    name = Column(String(255), nullable=False)
    role = Column(String(100))
    email = Column(String(255), unique=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    project = relationship("Project", back_populates="team_members")
    assigned_tasks = relationship("Task", back_populates="assignee")
    authored_snippets = relationship("CodeSnippet", back_populates="author")