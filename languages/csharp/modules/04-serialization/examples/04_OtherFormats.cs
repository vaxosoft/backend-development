using System.Web;
using System.Xml;
using System.Xml.Serialization;
using CsvHelper;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace M04.Examples;

[CsvRecord]
public sealed partial record TaskRow(int Id, string Title, string Status, [CsvColumn("due_date")] string? DueDate);

/// <summary>CSV (CsvHelper), YAML (YamlDotNet), XML (XmlSerializer) and form-encoded bodies.</summary>
public static class OtherFormats
{
    public sealed class TaskDoc
    {
        public string Title { get; set; } = "";

        public string Status { get; set; } = "todo";

        public List<string> Tags { get; set; } = [];
    }

    [XmlRoot("task")]
    public sealed class TaskXml
    {
        [XmlAttribute("id")]
        public int Id { get; set; }

        [XmlElement("title")]
        public string Title { get; set; } = "";

        [XmlArray("tags")]
        [XmlArrayItem("tag")]
        public List<string> Tags { get; set; } = [];
    }

    public static Task<int> RunAsync(string[] args)
    {
        Csv();
        Yaml();
        Xml();
        Form();
        return Task.FromResult(0);
    }

    private static void Csv()
    {
        TaskRow[] rows = [new(1, "Write docs", "todo", "2030-01-31"), new(2, "Fix, escape \"quotes\"", "done", null)];

        using var writer = new StringWriter();
        using (var csv = CsvWriter.Create(writer))
        {
            csv.WriteRecords(rows);
        }

        var text = writer.ToString();
        Console.Write("--- csv\n" + text.ReplaceLineEndings("\n"));

        using var csvReader = CsvReader.FromText(text);
        foreach (var row in csvReader.GetRecords<TaskRow>())
        {
            Console.WriteLine($"read back: {row}");
        }
    }

    private static void Yaml()
    {
        var serializer = new SerializerBuilder().WithNamingConvention(UnderscoredNamingConvention.Instance).Build();
        var deserializer = new DeserializerBuilder().WithNamingConvention(UnderscoredNamingConvention.Instance).Build();

        var text = serializer.Serialize(new TaskDoc { Title = "Write docs", Status = "in_progress", Tags = ["docs", "p1"] });
        Console.Write("--- yaml\n" + text.ReplaceLineEndings("\n"));

        var doc = deserializer.Deserialize<TaskDoc>("title: From YAML\nstatus: done\ntags: [a, b]\n");
        Console.WriteLine($"read back: {doc.Title} / {doc.Status} / {string.Join(',', doc.Tags)}");
    }

    private static void Xml()
    {
        var serializer = new XmlSerializer(typeof(TaskXml));
        using var writer = new StringWriter();
        serializer.Serialize(writer, new TaskXml { Id = 1, Title = "Write docs", Tags = ["docs", "p1"] }, new XmlSerializerNamespaces([XmlQualifiedName.Empty]));
        Console.WriteLine("--- xml\n" + writer.ToString().Replace("\r\n", "\n"));

        var back = (TaskXml)serializer.Deserialize(new StringReader("<task id=\"7\"><title>x</title><tags><tag>a</tag></tags></task>"))!;
        Console.WriteLine($"read back: {back.Id} {back.Title} {string.Join(',', back.Tags)}");
    }

    private static void Form()
    {
        var body = $"title={Uri.EscapeDataString("Fish & chips")}&tag=a&tag=b&due_date=2030-01-31";
        Console.WriteLine("--- form\n" + body);

        var form = HttpUtility.ParseQueryString(body);
        Console.WriteLine($"title={form["title"]} tags={string.Join(',', form.GetValues("tag")!)}");
    }
}
