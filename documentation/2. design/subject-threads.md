# Threads about a subject

CM34, 2026-10-20. Owner, member app decisions 41–43: chat on forms lives in
comm, as threads that name the form or the question they are about. comm
names no domain, so it never learns the words *form* or *question*: it
stores conversations about a **subject**, named by an opaque kind and id,
and the host says who may read them.

## The objects

Two roles, declared in `comm.blueprint.json`:

| Role | Holds |
| --- | --- |
| `subject_thread` | `subject_kind`, `subject_id`, `audience` (`private` or `group`), `owner_id`, `created_at` |
| `subject_message` | `thread_id`, `author_id`, `by_answerer`, `regarding_id`, `body`, `created_at` |

- **The subject is opaque.** `subject_kind` is the host's own word for what
  the subject is — `form` and `question` on the civic deployment — held only
  to the `subject_kind` pattern (`^[a-z][a-z0-9_]{0,63}$`). It is a plain
  string rather than an enum because comm owns no value of it: an enum must
  declare at least one, and any value comm declared would be a domain word.
  The host's standing resolver is what refuses a kind it does not know.
- **A private thread is one actor's.** It is keyed by subject and
  `owner_id`; a group thread has an empty `owner_id`, so a subject has at
  most one. The unique index `subject_audience_owner` is what makes opening
  idempotent, including under a race.
- **`regarding_id` is a finer reference inside the subject**, opaque and
  optional. The member app's group chat on a form (MF30) is one thread per
  form, with each message tagged with the question it is about or with none
  for the whole form; `regarding_id` carries that tag. Empty means the
  subject as a whole.
- The civic spec names them `form_thread`/`form_message`; the school example
  names them `homework_query`/`homework_reply`, so the two-domain floor holds.

## Who reads a thread

comm does not know who was sent a form or who speaks for the agency, so the
host tells it, through one seam on the manager:

```go
type Standing func(ctx context.Context, actorID, subjectKind, subjectID string) (models.SubjectStanding, error)

func (m *CommManager) UseStanding(s Standing)
```

It answers one of three standings for one caller on one subject:

| Standing | Meaning | Reads |
| --- | --- | --- |
| `StandingNone` | the subject is nothing to this caller, or does not exist | nothing |
| `StandingAudience` | the subject reached this caller | the group thread and their own private thread |
| `StandingAnswerer` | this caller answers for whoever sent the subject | every thread on it |

The rules it is held to, each tested through a real mux with a real caller in
`routes/subject_fence_security_test.go`, and each of the first three
mutation-checked:

- **404, never 403.** Another actor's private thread, a subject that never
  reached the caller, a stranger, an empty caller and a thread that was never
  opened all answer `ErrSubjectNotFound` with the same text.
- **Closed when nothing is supplied.** A manager with no `Standing` fences
  everybody out, answerer included.
- **A failed lookup is a 500**, through `dispatch`'s redaction, never the
  host's reason and never a 404.
- **The session is the author.** The thread comes from the path and the
  author from `{"from": "caller"}`; a body naming an `author_id`,
  `by_answerer` or `thread_id` is ignored. Whether a message is the answering
  side's is the standing at the time it was posted, never the body's claim.

**Answerer wins.** A caller the host says answers for a subject is treated as
the answering side even if the subject also reached them, so an answerer
cannot open a private thread on it (a 400): a private thread is opened by the
one it is with.

**Standing is asked on every read.** A member whose standing lapses — the
form was withdrawn, they left the group it was sent to — stops reading even
their own private thread. That follows forms' own rule that a resident sees
only the forms sent to their groups (forms F15).

## A reply is signed by the answering side

The member app signs a reply "the agency", never a named person, because
staff or an agent may answer (MF29). comm enforces that by omission rather
than leaving it to the client: a message posted with answerer standing is
stored with its author, and `SubjectThreadMessage.For(reader)` blanks
`author_id` for every reader who is not themselves an answerer. The answering
side still sees who of them wrote it.

## Counts and notifications reuse what comm has

- **Message counts.** Listing a subject's threads returns each with its
  `message_count` and `last_message_at`, read over the `thread_created`
  index — what MF30's "Group chat (n)" link shows.
- **Unread is the Inbox's.** A reply raises a notification through
  `NotificationStore.Raise` in the same transaction as the message, so the
  unread badge, mark-read and muting are the ones notifications already have.
  The event is comm's own, `thread_reply`, under comm's own category `chat`,
  with `subject_kind: subject_thread` and the thread's id, so the Inbox row
  opens the thread. It never carries an `author_actor_id`, for the reason
  above.
- **Who is told.** On a private thread, the one it is with, when the
  answering side posts; and, when the one it is with posts, every actor the
  host's `Answerers` names for the subject (below). On a group thread, every
  actor from the subject's audience who has posted there before, except the
  poster; the answering side is never told of a group post. Nobody is told
  of their own message. A recipient who muted `chat` is skipped and the post
  still lands; any other failure rolls the message back with it.
- **Pushed like any other notification.** A reply's notifications are
  raised inside the message's transaction, so they are handed to Web Push
  (CM35) only after it commits; a rolled-back post pushes nothing.

## Telling the answering side

CM36, 2026-10-20. A private thread can be about a subject whose answering
side are people with an Inbox of their own — a join request, decided by a
group's leaders or an agency's owner (member app decision 63). For those a
reply from the one applying has to reach the deciders, and comm cannot know
who they are. The host names them through a second, optional seam:

```go
type Answerers func(ctx context.Context, subjectKind, subjectID string) ([]string, error)

func (m *CommManager) UseAnswerers(a Answerers)
```

- **Asked only for a private-thread post by the one it is with**, before the
  transaction opens. Every named actor is told once, the poster never, and an
  empty id never.
- **Each name is held to the standing seam.** A named actor `Standing` does
  not call an answerer on that subject is skipped, so the two seams cannot
  disagree into telling somebody about a thread they cannot open.
- **No seam, or no names, tells nobody** — CM34's behaviour, and still the
  answer for a subject kind the host returns nothing for.
- **A form's private chat reaches the agency** (CM38, owner, member app
  decision 82). comm needed no change for it: the host names the agency's
  answering side for its form and question kinds through this same seam, and
  a member's post raises the same `thread_reply` a join request's applicant
  raises. Which actors that is, and when a member is the audience who can
  post at all, are the host's (`mwanachama-wakala-api`'s comm-mount.md).
- **A failed lookup is a 500 and stores nothing**, like a failed standing.

## A join request as a subject

Nothing in comm names a join request. The host picks its own
`subject_kind` for each kind of request (an agency's, a group's), and wires
the two seams:

| Seam | Answers for a join request |
| --- | --- |
| `Standing` | *audience* for the person applying, *answerer* for whoever decides it, *none* for anybody else |
| `Answerers` | whoever decides it, so a message from the applicant reaches their Inbox |

The applicant opens the private thread, as with a form. A decider cannot
open one (a private thread is opened by the one it is with), so a host that
wants the chat to exist before the applicant writes opens it in-process on
the applicant's behalf when the request is filed. The applicant reads
replies signed by the answering side, without a named author. Who the
deciders are, and admitting an applicant who is not yet a member to the
comm mount, are `mwanachama-wakala-api`'s (WK110).

## What is not here

- **The answering side's own inbox, unless the host names it.** Without
  `UseAnswerers`, a member's message on a private thread raises nothing,
  because comm has no actor list for "the agency"; the host names it per
  subject kind (CM36, CM38).
- **Per-thread read marks.** Unread is the notification badge; there is no
  "read up to here" per thread.
- **The host's resolver.** Which subject kinds exist, who a form reached and
  who answers for it are `mwanachama-wakala-api`'s to wire (its WK101).
