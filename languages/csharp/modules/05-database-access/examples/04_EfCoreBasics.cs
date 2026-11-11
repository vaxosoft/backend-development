using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

namespace M05.Examples;

/// <summary>EF Core: DbContext as a unit of work, change tracking, AsNoTracking, Include vs N+1.</summary>
public static class EfCoreBasics
{
    public sealed class TaskEntity
    {
        public int Id { get; set; }

        public required string Title { get; set; }

        public DateOnly? DueDate { get; set; }

        public List<TagEntity> Tags { get; set; } = [];
    }

    public sealed class TagEntity
    {
        public int Id { get; set; }

        public int TaskEntityId { get; set; }

        public required string Name { get; set; }
    }

    public sealed class TrackerDb(DbContextOptions<TrackerDb> options) : DbContext(options)
    {
        public DbSet<TaskEntity> Tasks => Set<TaskEntity>();

        public DbSet<TagEntity> Tags => Set<TagEntity>();

        protected override void OnModelCreating(ModelBuilder modelBuilder) =>
            modelBuilder.Entity<TaskEntity>().Property(t => t.Title).HasMaxLength(200);
    }

    public static async Task<int> RunAsync(string[] args)
    {
        // Keep one connection open so the in-memory database survives between DbContext instances.
        await using var keeper = new SqliteConnection("Data Source=:memory:");
        await keeper.OpenAsync();
        var options = new DbContextOptionsBuilder<TrackerDb>().UseSqlite(keeper).Options;

        await using (var db = new TrackerDb(options))
        {
            await db.Database.EnsureCreatedAsync();
            db.Tasks.AddRange(
                new TaskEntity { Title = "Write docs", DueDate = new DateOnly(2030, 1, 31), Tags = [new() { Name = "docs" }, new() { Name = "p1" }] },
                new TaskEntity { Title = "Fix bug", Tags = [new() { Name = "bug" }] });
            Console.WriteLine($"saved {await db.SaveChangesAsync()} rows");
        }

        // A short-lived context per unit of work: change tracking notices the edit.
        await using (var db = new TrackerDb(options))
        {
            var task = await db.Tasks.OrderBy(t => t.Id).FirstAsync();
            task.Title = "Write the docs";
            Console.WriteLine($"state before save: {db.Entry(task).State}");
            await db.SaveChangesAsync();
            Console.WriteLine($"state after save : {db.Entry(task).State}");
        }

        await using (var db = new TrackerDb(options))
        {
            var tracked = await db.Tasks.ToListAsync();
            Console.WriteLine($"tracked entities  : {db.ChangeTracker.Entries().Count()} (of {tracked.Count})");

            db.ChangeTracker.Clear();
            var readOnly = await db.Tasks.AsNoTracking().ToListAsync();
            Console.WriteLine($"AsNoTracking      : {db.ChangeTracker.Entries().Count()} (of {readOnly.Count})");
        }

        await using (var db = new TrackerDb(options))
        {
            var query = db.Tasks.AsNoTracking().Include(t => t.Tags).Where(t => t.DueDate != null).OrderBy(t => t.Title);
            foreach (var t in await query.ToListAsync())
            {
                Console.WriteLine($"{t.Title}: {string.Join(',', t.Tags.Select(x => x.Name))} (tags loaded by one Include)");
            }

            var count = await db.Tags.CountAsync(t => t.Name == "bug");
            Console.WriteLine($"tasks tagged bug (COUNT runs in SQL): {count}");
        }

        return 0;
    }
}
