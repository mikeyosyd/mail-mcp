function run(argv) {
  const Mail = Application("Mail");
  Mail.includeStandardAdditions = true;

  // 1. CRITICAL: Check if running FIRST
  if (!Mail.running()) {
    return JSON.stringify({
      success: false,
      error: "Mail.app is not running. Please start Mail.app and try again.",
      errorCode: "MAIL_APP_NOT_RUNNING",
    });
  }

  // 2. Logging setup
  const logs = [];
  function log(message) {
    logs.push(message);
  }

  // 3. Argument Parsing & Validation
  let args;
  try {
    args = JSON.parse(argv[0]);
  } catch (e) {
    return JSON.stringify({
      success: false,
      error: "Failed to parse input arguments JSON",
      logs: logs.join("\n"),
    });
  }

  const accountName = args.account;
  const mailboxPath = args.mailboxPath;
  const dryRun = args.dryRun === true;

  if (!accountName || !mailboxPath || !mailboxPath.length) {
    return JSON.stringify({
      success: false,
      error: "account and mailboxPath (at least one element) are required.",
      errorCode: "MISSING_PARAMETERS",
      logs: logs.join("\n"),
    });
  }

  // 4. Execution wrapped in try/catch
  try {
    const targetAccount = Mail.accounts.byName(accountName);
    try {
      targetAccount.name();
    } catch (e) {
      return JSON.stringify({
        success: false,
        error: `Account "${accountName}" not found.`,
        logs: logs.join("\n"),
      });
    }

    // Robust mailbox traversal function (same as find_messages.js)
    function findMailboxByPath(account, targetPath) {
      if (!targetPath || targetPath.length === 0) return account;

      try {
        let current = account;
        for (let i = 0; i < targetPath.length; i++) {
          const part = targetPath[i];
          let next = null;
          try {
            next = current.mailboxes.whose({ name: part })()[0];
          } catch (e) {}

          if (!next) {
            try {
              next = current.mailboxes[part];
              next.name();
            } catch (e) {}
          }
          if (!next) throw new Error("not found");
          current = next;
        }
        return current;
      } catch (e) {}

      try {
        const allMailboxes = account.mailboxes();
        for (let i = 0; i < allMailboxes.length; i++) {
          const mbx = allMailboxes[i];
          const path = [];
          let current = mbx;
          while (current) {
            try {
              const name = current.name();
              if (name === account.name()) break;
              path.unshift(name);
              current = current.container();
            } catch (e) {
              break;
            }
          }
          if (path.length === targetPath.length) {
            let match = true;
            for (let j = 0; j < path.length; j++) {
              if (path[j] !== targetPath[j]) {
                match = false;
                break;
              }
            }
            if (match) return mbx;
          }
        }
      } catch (e) {}
      return null;
    }

    const parentPath = mailboxPath.slice(0, -1);
    const name = mailboxPath[mailboxPath.length - 1];

    const parent = findMailboxByPath(targetAccount, parentPath);
    if (!parent) {
      return JSON.stringify({
        success: false,
        error: `Parent mailbox "${parentPath.join(" > ")}" not found in account "${accountName}". Create it first.`,
        logs: logs.join("\n"),
      });
    }

    // Already there? (exact, case-sensitive name under this parent)
    let existing = null;
    try {
      const found = parent.mailboxes.whose({ name: name })();
      if (found && found.length > 0) existing = found[0];
    } catch (e) {}

    const base = {
      dry_run: dryRun,
      account: accountName,
      mailbox_path: mailboxPath,
      parent_path: parentPath,
    };

    if (existing) {
      log(`Mailbox "${mailboxPath.join(" > ")}" already exists.`);
      return JSON.stringify({
        success: true,
        data: Object.assign(base, { status: "exists", message: `Mailbox "${mailboxPath.join(" > ")}" already exists; nothing to do.` }),
        logs: logs.join("\n"),
      });
    }

    if (dryRun) {
      return JSON.stringify({
        success: true,
        data: Object.assign(base, { status: "would_create", message: `Dry run: mailbox "${mailboxPath.join(" > ")}" would be created.` }),
        logs: logs.join("\n"),
      });
    }

    // Create: the JXA equivalent of AppleScript's `make new mailbox with properties {name:...} at parent`
    const mb = Mail.Mailbox({ name: name });
    parent.mailboxes.push(mb);
    log(`Created mailbox "${mailboxPath.join(" > ")}".`);

    // Verify
    let verified = false;
    try {
      const again = parent.mailboxes.whose({ name: name })();
      verified = !!(again && again.length > 0);
    } catch (e) {}

    return JSON.stringify({
      success: true,
      data: Object.assign(base, {
        status: "created",
        verified: verified,
        message: `Mailbox "${mailboxPath.join(" > ")}" created${verified ? "" : " (not yet visible; Mail may still be syncing it)"}.`,
      }),
      logs: logs.join("\n"),
    });
  } catch (e) {
    log(`Error creating mailbox: ${e.toString()}`);
    return JSON.stringify({
      success: false,
      error: `Failed to create mailbox: ${e.toString()}`,
      logs: logs.join("\n"),
    });
  }
}
