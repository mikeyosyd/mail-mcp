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
  const targetMailboxPath = args.targetMailboxPath;
  const messageIds = args.messageIds;
  const dryRun = args.dryRun === true;

  if (!accountName || !mailboxPath || !targetMailboxPath || !messageIds) {
    return JSON.stringify({
      success: false,
      error: "account, mailboxPath, targetMailboxPath and messageIds are required.",
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

    const sourceMailbox = findMailboxByPath(targetAccount, mailboxPath);
    if (!sourceMailbox) {
      return JSON.stringify({
        success: false,
        error: `Mailbox "${mailboxPath.join(" > ")}" not found in account "${accountName}".`,
        logs: logs.join("\n"),
      });
    }

    const targetMailbox = findMailboxByPath(targetAccount, targetMailboxPath);
    if (!targetMailbox) {
      return JSON.stringify({
        success: false,
        error: `Target mailbox "${targetMailboxPath.join(" > ")}" not found in account "${accountName}".`,
        logs: logs.join("\n"),
      });
    }

    log(
      `Moving ${messageIds.length} message(s) from "${mailboxPath.join(" > ")}" to "${targetMailboxPath.join(" > ")}"${dryRun ? " (dry run)" : ""}.`,
    );

    const succeeded = [];
    const notFound = [];
    const failed = [];

    for (let i = 0; i < messageIds.length; i++) {
      const id = messageIds[i];
      let msg = null;
      try {
        // whose() is constant time versus a linear scan (see get_message_content.js)
        const matches = sourceMailbox.messages.whose({ id: id })();
        if (matches && matches.length > 0) msg = matches[0];
      } catch (e) {
        log(`Lookup failed for ID ${id}: ${e.toString()}`);
      }

      if (!msg) {
        notFound.push(id);
        continue;
      }

      let subject = "";
      let sender = "";
      try {
        subject = msg.subject() || "";
      } catch (e) {}
      try {
        sender = msg.sender() || "";
      } catch (e) {}

      if (dryRun) {
        succeeded.push({ id: id, subject: subject, sender: sender });
        continue;
      }

      try {
        // Setting the mailbox property moves the message (AppleScript: move message to mailbox)
        msg.mailbox = targetMailbox;
        succeeded.push({ id: id, subject: subject, sender: sender });
      } catch (e) {
        log(`Move failed for ID ${id}: ${e.toString()}`);
        failed.push({ id: id, error: e.toString() });
      }
    }

    log(
      `Done: ${succeeded.length} ${dryRun ? "would be moved" : "moved"}, ${notFound.length} not found, ${failed.length} failed.`,
    );

    return JSON.stringify({
      success: true,
      data: {
        dry_run: dryRun,
        account: accountName,
        source_mailbox_path: mailboxPath,
        target_mailbox_path: targetMailboxPath,
        requested: messageIds.length,
        moved: dryRun ? [] : succeeded,
        would_move: dryRun ? succeeded : [],
        not_found: notFound,
        failed: failed,
        message: dryRun
          ? `Dry run: ${succeeded.length} of ${messageIds.length} message(s) would be moved.`
          : `${succeeded.length} of ${messageIds.length} message(s) moved.`,
      },
      logs: logs.join("\n"),
    });
  } catch (e) {
    log(`Error moving messages: ${e.toString()}`);
    return JSON.stringify({
      success: false,
      error: `Failed to move messages: ${e.toString()}`,
      logs: logs.join("\n"),
    });
  }
}
