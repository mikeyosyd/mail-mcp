function run(argv) {
  const Mail = Application("Mail");
  Mail.includeStandardAdditions = true;
  ObjC.import("Foundation");

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
  const messageId = args.messageId;
  const directory = args.directory; // absolute, already expanded and created by the Go side
  const attachmentId = args.attachmentId || null;
  const attachmentName = args.attachmentName || null;
  const overwrite = args.overwrite === true;
  const dryRun = args.dryRun === true;

  if (!accountName || !mailboxPath || !messageId || !directory) {
    return JSON.stringify({
      success: false,
      error: "account, mailboxPath, messageId and directory are required.",
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

    const targetMailbox = findMailboxByPath(targetAccount, mailboxPath);
    if (!targetMailbox) {
      return JSON.stringify({
        success: false,
        error: `Mailbox "${mailboxPath.join(" > ")}" not found in account "${accountName}".`,
        logs: logs.join("\n"),
      });
    }

    // whose() is constant time versus a linear scan (see get_message_content.js)
    const matches = targetMailbox.messages.whose({ id: messageId })();
    if (!matches || matches.length === 0) {
      return JSON.stringify({
        success: false,
        error: `Message with ID ${messageId} not found in mailbox "${mailboxPath.join(" > ")}". The message may have been deleted or moved.`,
        logs: logs.join("\n"),
      });
    }
    const msg = matches[0];
    let subject = "";
    try {
      subject = msg.subject() || "";
    } catch (e) {}

    const fm = $.NSFileManager.defaultManager;
    const saved = [];
    const wouldSave = [];
    const skipped = [];
    const failed = [];
    let considered = 0;

    const attachments = msg.mailAttachments();
    log(`Message ${messageId} has ${attachments.length} attachment(s).`);

    for (let i = 0; i < attachments.length; i++) {
      const att = attachments[i];
      const info = { index: i };
      try {
        info.id = att.id();
      } catch (e) {
        info.id = null;
      }
      try {
        info.name = att.name() || `attachment-${i}`;
      } catch (e) {
        info.name = `attachment-${i}`;
      }
      try {
        info.mimeType = att.mimeType();
      } catch (e) {
        info.mimeType = null;
      }
      try {
        info.fileSize = att.fileSize();
      } catch (e) {
        info.fileSize = 0;
      }
      try {
        info.downloaded = att.downloaded();
      } catch (e) {
        info.downloaded = false;
      }

      // Selection: by id, by exact name, or all
      if (attachmentId && info.id !== attachmentId) continue;
      if (attachmentName && info.name !== attachmentName) continue;
      considered++;

      // Never let an attachment name escape the target directory
      const safeName = String(info.name)
        .replace(/[\/\\:]/g, "_")
        .replace(/^\.+/, "_");
      info.path = directory.replace(/\/+$/, "") + "/" + safeName;

      if (!info.downloaded) {
        info.reason = "not_downloaded";
        skipped.push(info);
        continue;
      }
      const exists = fm.fileExistsAtPath(info.path);
      if (exists && !overwrite) {
        info.reason = "exists";
        skipped.push(info);
        continue;
      }
      if (dryRun) {
        wouldSave.push(info);
        continue;
      }
      try {
        // Mail's generic `save` command: save <specifier> in <file>
        Mail.save(att, { in: Path(info.path) });
        saved.push(info);
      } catch (e) {
        log(`Save failed for "${info.name}": ${e.toString()}`);
        info.error = e.toString();
        failed.push(info);
      }
    }

    if (considered === 0 && (attachmentId || attachmentName)) {
      return JSON.stringify({
        success: false,
        error: `No attachment matched ${attachmentId ? "id " + attachmentId : 'name "' + attachmentName + '"'} on message ${messageId}.`,
        logs: logs.join("\n"),
      });
    }

    log(
      `Done: ${saved.length} saved, ${wouldSave.length} would save, ${skipped.length} skipped, ${failed.length} failed.`,
    );

    return JSON.stringify({
      success: true,
      data: {
        dry_run: dryRun,
        account: accountName,
        mailbox_path: mailboxPath,
        message_id: messageId,
        subject: subject,
        directory: directory,
        attachment_count: attachments.length,
        saved: saved,
        would_save: wouldSave,
        skipped: skipped,
        failed: failed,
        message: dryRun
          ? `Dry run: ${wouldSave.length} attachment(s) would be saved to ${directory}.`
          : `${saved.length} attachment(s) saved to ${directory}.`,
      },
      logs: logs.join("\n"),
    });
  } catch (e) {
    log(`Error saving attachments: ${e.toString()}`);
    return JSON.stringify({
      success: false,
      error: `Failed to save attachments: ${e.toString()}`,
      logs: logs.join("\n"),
    });
  }
}
