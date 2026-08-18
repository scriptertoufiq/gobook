# GoBook

A small social site. People sign up, write posts, read what everyone else has
written, and react to it.

It is built as two halves that work together: a service that holds the accounts
and the posts, and a web app people actually look at. Either half can be worked
on without disturbing the other.

---

## What you can do

### Accounts

Anyone can create an account with a name, an email address and a password.
After that they can sign in, sign out, and change their password — which always
asks for the current one first, so someone borrowing an unlocked laptop cannot
quietly take the account over.

People who forget their password can ask for a reset link by email. The reply is
the same whether or not the address has an account, so the form cannot be used
to find out who is registered here. Following the link leads to a page where a
new password is chosen. Setting it signs out every other device, on the
assumption that a forgotten password is sometimes a stolen one.

Email addresses can be made to require confirmation before an account is
allowed to do anything. That is switched off by default and can be turned on
without changing anything else.

There are two kinds of account. Most people are ordinary members. Administrators
can additionally see the full list of members, create accounts, and remove them.

### Posts

Signed-in members can write a post with a title and a body, and it appears at
the top of the feed immediately.

Everyone signed in can read every post. **Only the person who wrote a post can
change or remove it** — administrators included. That is a deliberate decision
rather than an oversight: nobody edits somebody else's words here.

Removing a post hides it from the site but does not scrub it from the records,
so it can be recovered if it turns out to have been a mistake.

The feed shows newest first and loads more as you scroll, so there is no paging
to click through. Posts can also be searched and sorted, and every post has its
own page with a proper address — so a post can be linked to, bookmarked, or
reopened later.

### Reactions

Posts can be reacted to with one of five responses: **like, love, care, sad, or
angry**. Choosing the same one again takes it back.

Reactions currently live in the browser they were made in. They survive
closing the tab, but nobody else can see them and they do not follow you to
another device.

---

## The two halves

**The service** holds everything real: the accounts, the posts, who wrote what,
and who is allowed to do what. Every rule is enforced here. If someone bypasses
the web app entirely and talks to the service directly, they still cannot edit a
post they did not write.

**The web app** is what people see. Sign-in and sign-up screens, the feed,
individual post pages, and an account page. Administrators additionally get the
member list.

The web app never decides what is allowed — it only shows or hides things to
keep the screen tidy. Hiding the edit button on someone else's post is a
courtesy; the service refuses the request regardless.

---

## Speed

Recently-read posts are kept close at hand so opening the same post twice does
not mean fetching it twice. This happens quietly in the background and keeps
itself honest: writing a post files it away immediately, editing one replaces
what was filed, and deleting one throws it out. Nobody is ever shown an old copy
of something that has since changed.

If that layer is switched off, or stops responding, the site carries on working
— it just does more work per page.

---

## Not built yet

Being straight about the edges:

- **Names on other people's posts.** Your own posts show your name. Everyone
  else's currently show as a number, because a post carries who wrote it but not
  their name yet.
- **Comments.** The button is there and opens the post, but there is nowhere to
  leave a comment.
- **Sharing.** The button does nothing.
- **Photos and video.** Posts are text only. The buttons that suggest otherwise
  are decoration.
- **Shared reactions.** As above, reactions do not leave your browser.
- **The side panels** on the home page — shortcuts, suggestions, contacts — are
  placeholders with invented content.

---

## Setting it up

Everything about running, configuring and extending this — including how to add
new kinds of content beyond posts — is in **[docs/DEVELOPING.md](docs/DEVELOPING.md)**.
